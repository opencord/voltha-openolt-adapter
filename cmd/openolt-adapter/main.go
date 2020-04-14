/*
* Copyright 2018-present Open Networking Foundation

* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at

* http://www.apache.org/licenses/LICENSE-2.0

* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

//Package main invokes the application
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/opencord/voltha-lib-go/v3/pkg/adapters/adapterif"

	"github.com/opencord/voltha-lib-go/v3/pkg/adapters"
	com "github.com/opencord/voltha-lib-go/v3/pkg/adapters/common"
	conf "github.com/opencord/voltha-lib-go/v3/pkg/config"
	"github.com/opencord/voltha-lib-go/v3/pkg/db/kvstore"
	"github.com/opencord/voltha-lib-go/v3/pkg/kafka"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-lib-go/v3/pkg/probe"
	"github.com/opencord/voltha-lib-go/v3/pkg/version"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	ac "github.com/opencord/voltha-openolt-adapter/internal/pkg/core"
	ic "github.com/opencord/voltha-protos/v3/go/inter_container"
	"github.com/opencord/voltha-protos/v3/go/voltha"
)

type adapter struct {
	instanceID       string
	config           *config.AdapterFlags
	iAdapter         adapters.IAdapter
	kafkaClient      kafka.Client
	kvClient         kvstore.Client
	kip              kafka.InterContainerProxy
	coreProxy        adapterif.CoreProxy
	adapterProxy     adapterif.AdapterProxy
	eventProxy       adapterif.EventProxy
	halted           bool
	exitChannel      chan int
	receiverChannels []<-chan *ic.InterContainerMessage
}

func newAdapter(cf *config.AdapterFlags) *adapter {
	var a adapter
	a.instanceID = cf.InstanceID
	a.config = cf
	a.halted = false
	a.exitChannel = make(chan int, 1)
	a.receiverChannels = make([]<-chan *ic.InterContainerMessage, 0)
	return &a
}

func (a *adapter) start(ctx context.Context) {
	logger.Info("Starting Core Adapter components")
	var err error

	var p *probe.Probe
	if value := ctx.Value(probe.ProbeContextKey); value != nil {
		if _, ok := value.(*probe.Probe); ok {
			p = value.(*probe.Probe)
			p.RegisterService(
				"message-bus",
				"kv-store",
				"container-proxy",
				"core-request-handler",
				"register-with-core",
			)
		}
	}

	// Setup KV Client
	logger.Debugw("create-kv-client", log.Fields{"kvstore": a.config.KVStoreType})
	if err = a.setKVClient(); err != nil {
		logger.Fatalw("error-setting-kv-client", log.Fields{"error": err})
	}

	if p != nil {
		p.UpdateStatus("kv-store", probe.ServiceStatusRunning)
	}

	// Setup Log Config
	cm := conf.NewConfigManager(a.kvClient, a.config.KVStoreType, a.config.KVStoreHost, a.config.KVStorePort, a.config.KVStoreTimeout)
	go conf.StartLogLevelConfigProcessing(cm, ctx)

	// Setup Kafka Client
	if a.kafkaClient, err = newKafkaClient("sarama", a.config.KafkaAdapterHost, a.config.KafkaAdapterPort); err != nil {
		logger.Fatalw("Unsupported-common-client", log.Fields{"error": err})
	}

	if p != nil {
		p.UpdateStatus("message-bus", probe.ServiceStatusRunning)
	}

	// Start the common InterContainer Proxy - retries indefinitely
	if a.kip, err = a.startInterContainerProxy(ctx, -1); err != nil {
		logger.Fatal("error-starting-inter-container-proxy")
	}

	// Create the core proxy to handle requests to the Core
	a.coreProxy = com.NewCoreProxy(a.kip, a.config.Topic, a.config.CoreTopic)

	// Create the adaptor proxy to handle request between olt and onu
	a.adapterProxy = com.NewAdapterProxy(a.kip, "brcm_openomci_onu", a.config.CoreTopic)

	// Create the event proxy to post events to KAFKA
	a.eventProxy = com.NewEventProxy(com.MsgClient(a.kafkaClient), com.MsgTopic(kafka.Topic{Name: a.config.EventTopic}))

	// Create the open OLT adapter
	if a.iAdapter, err = a.startOpenOLT(ctx, a.kip, a.coreProxy, a.adapterProxy, a.eventProxy, a.config); err != nil {
		logger.Fatalw("error-starting-openolt", log.Fields{"error": err})
	}

	// Register the core request handler
	if err = a.setupRequestHandler(ctx, a.instanceID, a.iAdapter); err != nil {
		logger.Fatalw("error-setting-core-request-handler", log.Fields{"error": err})
	}

	// Register this adapter to the Core - retries indefinitely
	if err = a.registerWithCore(ctx, -1); err != nil {
		logger.Fatal("error-registering-with-core")
	}

	// check the readiness and liveliness and update the probe status
	a.checkServicesReadiness(ctx)
}

/**
This function checks the liveliness and readiness of the kakfa and kv-client services
and update the status in the probe.
*/
func (a *adapter) checkServicesReadiness(ctx context.Context) {
	// checks the kafka readiness
	go a.checkKafkaReadiness(ctx)

	// checks the kv-store readiness
	go a.checkKvStoreReadiness(ctx)
}

/**
This function checks the liveliness and readiness of the kv-store service
and update the status in the probe.
*/
func (a *adapter) checkKvStoreReadiness(ctx context.Context) {
	// dividing the live probe interval by 2 to get updated status every 30s
	timeout := a.config.LiveProbeInterval / 2
	kvStoreChannel := make(chan bool, 1)

	// Default false to check the liveliness.
	kvStoreChannel <- false
	for {
		timeoutTimer := time.NewTimer(timeout)
		select {
		case liveliness := <-kvStoreChannel:
			if !liveliness {
				// kv-store not reachable or down, updating the status to not ready state
				probe.UpdateStatusFromContext(ctx, "kv-store", probe.ServiceStatusNotReady)
				timeout = a.config.NotLiveProbeInterval
			} else {
				// kv-store is reachable , updating the status to running state
				probe.UpdateStatusFromContext(ctx, "kv-store", probe.ServiceStatusRunning)
				timeout = a.config.LiveProbeInterval / 2
			}
			// Check if the timer has expired or not
			if !timeoutTimer.Stop() {
				<-timeoutTimer.C
			}
		case <-timeoutTimer.C:
			// Check the status of the kv-store. Use timeout of 2 seconds to avoid forever blocking
			logger.Info("kv-store liveliness-recheck")
			timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Second)

			kvStoreChannel <- a.kvClient.IsConnectionUp(timeoutCtx)
			// Cleanup cancel func resources
			cancelFunc()
		}
	}
}

/**
This function checks the liveliness and readiness of the kafka service
and update the status in the probe.
*/
func (a *adapter) checkKafkaReadiness(ctx context.Context) {
	livelinessChannel := a.kafkaClient.EnableLivenessChannel(true)
	healthinessChannel := a.kafkaClient.EnableHealthinessChannel(true)
	timeout := a.config.LiveProbeInterval
	failed := false
	for {
		timeoutTimer := time.NewTimer(timeout)

		select {
		case healthiness := <-healthinessChannel:
			if !healthiness {
				// This will eventually cause K8s to restart the container, and will do
				// so in a way that allows cleanup to continue, rather than an immediate
				// panic and exit here.
				probe.UpdateStatusFromContext(ctx, "message-bus", probe.ServiceStatusFailed)
				failed = true
			}
			// Check if the timer has expired or not
			if !timeoutTimer.Stop() {
				<-timeoutTimer.C
			}
		case liveliness := <-livelinessChannel:
			if failed {
				// Failures of the message bus are permanent and can't ever be recovered from,
				// so make sure we never inadvertently reset a failed state back to unready.
			} else if !liveliness {
				// kafka not reachable or down, updating the status to not ready state
				probe.UpdateStatusFromContext(ctx, "message-bus", probe.ServiceStatusNotReady)
				timeout = a.config.NotLiveProbeInterval
			} else {
				// kafka is reachable , updating the status to running state
				probe.UpdateStatusFromContext(ctx, "message-bus", probe.ServiceStatusRunning)
				timeout = a.config.LiveProbeInterval
			}
			// Check if the timer has expired or not
			if !timeoutTimer.Stop() {
				<-timeoutTimer.C
			}
		case <-timeoutTimer.C:
			logger.Info("kafka-proxy-liveness-recheck")
			// send the liveness probe in a goroutine; we don't want to deadlock ourselves as
			// the liveness probe may wait (and block) writing to our channel.
			err := a.kafkaClient.SendLiveness()
			if err != nil {
				// Catch possible error case if sending liveness after Sarama has been stopped.
				logger.Warnw("error-kafka-send-liveness", log.Fields{"error": err})
			}
		}
	}
}

func (a *adapter) stop(ctx context.Context) {
	// Stop leadership tracking
	a.halted = true

	// send exit signal
	a.exitChannel <- 0

	// Cleanup - applies only if we had a kvClient
	if a.kvClient != nil {
		// Release all reservations
		if err := a.kvClient.ReleaseAllReservations(ctx); err != nil {
			logger.Infow("fail-to-release-all-reservations", log.Fields{"error": err})
		}
		// Close the DB connection
		a.kvClient.Close()
	}

	if a.kip != nil {
		a.kip.Stop()
	}

	// TODO:  More cleanup
}

func newKVClient(storeType, address string, timeout time.Duration) (kvstore.Client, error) {

	logger.Infow("kv-store-type", log.Fields{"store": storeType})
	switch storeType {
	case "consul":
		return kvstore.NewConsulClient(address, timeout)
	case "etcd":
		return kvstore.NewEtcdClient(address, timeout)
	}
	return nil, errors.New("unsupported-kv-store")
}

func newKafkaClient(clientType, host string, port int) (kafka.Client, error) {

	logger.Infow("common-client-type", log.Fields{"client": clientType})
	switch clientType {
	case "sarama":
		return kafka.NewSaramaClient(
			kafka.Host(host),
			kafka.Port(port),
			kafka.ProducerReturnOnErrors(true),
			kafka.ProducerReturnOnSuccess(true),
			kafka.ProducerMaxRetries(6),
			kafka.ProducerRetryBackoff(time.Millisecond*30),
			kafka.MetadatMaxRetries(15)), nil
	}

	return nil, errors.New("unsupported-client-type")
}

func (a *adapter) setKVClient() error {
	addr := a.config.KVStoreHost + ":" + strconv.Itoa(a.config.KVStorePort)
	client, err := newKVClient(a.config.KVStoreType, addr, a.config.KVStoreTimeout)
	if err != nil {
		a.kvClient = nil
		return err
	}
	a.kvClient = client

	return nil
}

func (a *adapter) startInterContainerProxy(ctx context.Context, retries int) (kafka.InterContainerProxy, error) {
	logger.Infow("starting-intercontainer-messaging-proxy", log.Fields{"host": a.config.KafkaAdapterHost,
		"port": a.config.KafkaAdapterPort, "topic": a.config.Topic})
	var err error
	kip := kafka.NewInterContainerProxy(
		kafka.InterContainerHost(a.config.KafkaAdapterHost),
		kafka.InterContainerPort(a.config.KafkaAdapterPort),
		kafka.MsgClient(a.kafkaClient),
		kafka.DefaultTopic(&kafka.Topic{Name: a.config.Topic}))
	count := 0
	for {
		if err = kip.Start(); err != nil {
			logger.Warnw("error-starting-messaging-proxy", log.Fields{"error": err})
			if retries == count {
				return nil, err
			}
			count = +1
			// Take a nap before retrying
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	probe.UpdateStatusFromContext(ctx, "container-proxy", probe.ServiceStatusRunning)
	logger.Info("common-messaging-proxy-created")
	return kip, nil
}

func (a *adapter) startOpenOLT(ctx context.Context, kip kafka.InterContainerProxy,
	cp adapterif.CoreProxy, ap adapterif.AdapterProxy, ep adapterif.EventProxy,
	cfg *config.AdapterFlags) (*ac.OpenOLT, error) {
	logger.Info("starting-open-olt")
	var err error
	sOLT := ac.NewOpenOLT(ctx, a.kip, cp, ap, ep, cfg)

	if err = sOLT.Start(ctx); err != nil {
		return nil, err
	}

	logger.Info("open-olt-started")
	return sOLT, nil
}

func (a *adapter) setupRequestHandler(ctx context.Context, coreInstanceID string, iadapter adapters.IAdapter) error {
	logger.Info("setting-request-handler")
	requestProxy := com.NewRequestHandlerProxy(coreInstanceID, iadapter, a.coreProxy)
	if err := a.kip.SubscribeWithRequestHandlerInterface(kafka.Topic{Name: a.config.Topic}, requestProxy); err != nil {
		return err

	}
	probe.UpdateStatusFromContext(ctx, "core-request-handler", probe.ServiceStatusRunning)
	logger.Info("request-handler-setup-done")
	return nil
}

func (a *adapter) registerWithCore(ctx context.Context, retries int) error {
	logger.Info("registering-with-core")
	adapterDescription := &voltha.Adapter{Id: "openolt", // Unique name for the device type
		Vendor:  "VOLTHA OpenOLT",
		Version: version.VersionInfo.Version}
	types := []*voltha.DeviceType{{Id: "openolt",
		Adapter:                     "openolt", // Name of the adapter that handles device type
		AcceptsBulkFlowUpdate:       false,     // Currently openolt adapter does not support bulk flow handling
		AcceptsAddRemoveFlowUpdates: true}}
	deviceTypes := &voltha.DeviceTypes{Items: types}
	count := 0
	for {
		if err := a.coreProxy.RegisterAdapter(context.TODO(), adapterDescription, deviceTypes); err != nil {
			logger.Warnw("registering-with-core-failed", log.Fields{"error": err})
			if retries == count {
				return err
			}
			count++
			// Take a nap before retrying
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	probe.UpdateStatusFromContext(ctx, "register-with-core", probe.ServiceStatusRunning)
	logger.Info("registered-with-core")
	return nil
}

func waitForExit() int {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	exitChannel := make(chan int)

	go func() {
		s := <-signalChannel
		switch s {
		case syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT:
			logger.Infow("closing-signal-received", log.Fields{"signal": s})
			exitChannel <- 0
		default:
			logger.Infow("unexpected-signal-received", log.Fields{"signal": s})
			exitChannel <- 1
		}
	}()

	code := <-exitChannel
	return code
}

func printBanner() {
	fmt.Println(`   ____                     ____  _   _______ `)
	fmt.Println(`  / _  \                   / __ \| | |__   __|`)
	fmt.Println(` | |  | |_ __   ___ _ __  | |  | | |    | |   `)
	fmt.Println(` | |  | | '_ \ / _ \ '_ \ | |  | | |    | |   `)
	fmt.Println(` | |__| | |_) |  __/ | | || |__| | |____| |   `)
	fmt.Println(`  \____/| .__/ \___|_| |_| \____/|______|_|   `)
	fmt.Println(`        | |                                   `)
	fmt.Println(`        |_|                                   `)
	fmt.Println(`                                              `)
}

func printVersion() {
	fmt.Println("VOLTHA OpenOLT Adapter")
	fmt.Println(version.VersionInfo.String("  "))
}

func main() {
	start := time.Now()

	cf := config.NewAdapterFlags()
	cf.ParseCommandArguments()

	// Setup logging

	logLevel, err := log.StringToLogLevel(cf.LogLevel)
	if err != nil {
		logger.Fatalf("Cannot setup logging, %s", err)
	}

	// Setup default logger - applies for packages that do not have specific logger set
	if _, err := log.SetDefaultLogger(log.JSON, logLevel, log.Fields{"instanceId": cf.InstanceID}); err != nil {
		log.With(log.Fields{"error": err}).Fatal("Cannot setup logging")
	}

	// Update all loggers (provisionned via init) with a common field
	if err := log.UpdateAllLoggers(log.Fields{"instanceId": cf.InstanceID}); err != nil {
		log.With(log.Fields{"error": err}).Fatal("Cannot setup logging")
	}

	log.SetAllLogLevel(logLevel)

	realMain()

	defer log.CleanUp()

	// Print version / build information and exit
	if cf.DisplayVersionOnly {
		printVersion()
		return
	}

	// Print banner if specified
	if cf.Banner {
		printBanner()
	}

	logger.Infow("config", log.Fields{"config": *cf})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ad := newAdapter(cf)

	p := &probe.Probe{}
	go p.ListenAndServe(fmt.Sprintf("%s:%d", ad.config.ProbeHost, ad.config.ProbePort))

	probeCtx := context.WithValue(ctx, probe.ProbeContextKey, p)

	go ad.start(probeCtx)

	code := waitForExit()
	logger.Infow("received-a-closing-signal", log.Fields{"code": code})

	// Cleanup before leaving
	ad.stop(ctx)

	elapsed := time.Since(start)
	logger.Infow("run-time", log.Fields{"instanceId": ad.config.InstanceID, "time": elapsed / time.Second})
}
