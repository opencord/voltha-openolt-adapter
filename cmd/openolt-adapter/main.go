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
	"syscall"
	"time"

	conf "github.com/opencord/voltha-lib-go/v7/pkg/config"
	"github.com/opencord/voltha-lib-go/v7/pkg/db/kvstore"
	"github.com/opencord/voltha-lib-go/v7/pkg/events"
	"github.com/opencord/voltha-lib-go/v7/pkg/events/eventif"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"
	"github.com/opencord/voltha-lib-go/v7/pkg/kafka"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	"github.com/opencord/voltha-lib-go/v7/pkg/probe"
	"github.com/opencord/voltha-lib-go/v7/pkg/version"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	ac "github.com/opencord/voltha-openolt-adapter/internal/pkg/core"
	"github.com/opencord/voltha-protos/v5/go/adapter_service"
	"github.com/opencord/voltha-protos/v5/go/common"
	ca "github.com/opencord/voltha-protos/v5/go/core_adapter"
	"github.com/opencord/voltha-protos/v5/go/core_service"
	"github.com/opencord/voltha-protos/v5/go/health"
	"github.com/opencord/voltha-protos/v5/go/olt_inter_adapter_service"
	"github.com/opencord/voltha-protos/v5/go/voltha"
	"google.golang.org/grpc"
)

const (
	clusterMessagingService = "cluster-message-service"
	oltAdapterService       = "olt-adapter-service"
	kvService               = "kv-service"
	coreService             = "core-service"
)

type adapter struct {
	instanceID  string
	config      *config.AdapterFlags
	grpcServer  *vgrpc.GrpcServer
	oltAdapter  *ac.OpenOLT
	kafkaClient kafka.Client
	kvClient    kvstore.Client
	coreClient  *vgrpc.Client
	eventProxy  eventif.EventProxy
	halted      bool
	exitChannel chan int
}

func newAdapter(cf *config.AdapterFlags) *adapter {
	var a adapter
	a.instanceID = cf.InstanceID
	a.config = cf
	a.halted = false
	a.exitChannel = make(chan int, 1)
	return &a
}

func (a *adapter) start(ctx context.Context) {
	logger.Info(ctx, "Starting Core Adapter components")
	var err error

	var p *probe.Probe
	if value := ctx.Value(probe.ProbeContextKey); value != nil {
		if _, ok := value.(*probe.Probe); ok {
			p = value.(*probe.Probe)
			p.RegisterService(
				ctx,
				clusterMessagingService,
				kvService,
				oltAdapterService,
				coreService,
			)
		}
	}

	// Setup KV Client
	logger.Debugw(ctx, "create-kv-client", log.Fields{"kvstore": a.config.KVStoreType})
	if err = a.setKVClient(ctx); err != nil {
		logger.Fatalw(ctx, "error-setting-kv-client", log.Fields{"error": err})
	}

	if p != nil {
		p.UpdateStatus(ctx, kvService, probe.ServiceStatusRunning)
	}

	// Setup Log Config
	cm := conf.NewConfigManager(ctx, a.kvClient, a.config.KVStoreType, a.config.KVStoreAddress, a.config.KVStoreTimeout)

	go conf.StartLogLevelConfigProcessing(cm, ctx)
	go conf.StartLogFeaturesConfigProcessing(cm, ctx)

	// Setup Kafka Client
	if a.kafkaClient, err = newKafkaClient(ctx, "sarama", a.config.KafkaClusterAddress); err != nil {
		logger.Fatalw(ctx, "Unsupported-common-client", log.Fields{"error": err})
	}

	// Start kafka communication with the broker
	if err := kafka.StartAndWaitUntilKafkaConnectionIsUp(ctx, a.kafkaClient, a.config.HeartbeatCheckInterval, clusterMessagingService); err != nil {
		logger.Fatal(ctx, "unable-to-connect-to-kafka")
	}

	// Create the event proxy to post events to KAFKA
	a.eventProxy = events.NewEventProxy(events.MsgClient(a.kafkaClient), events.MsgTopic(kafka.Topic{Name: a.config.EventTopic}))
	go func() {
		if err := a.eventProxy.Start(); err != nil {
			logger.Fatalw(ctx, "event-proxy-cannot-start", log.Fields{"error": err})
		}
	}()

	// Create the Core client to handle requests to the Core.  Note that the coreClient is an interface and needs to be
	// cast to the appropriate grpc client by invoking GetCoreGrpcClient on the a.coreClient
	if a.coreClient, err = vgrpc.NewClient(
		a.config.AdapterEndpoint,
		a.config.CoreEndpoint,
		a.coreRestarted); err != nil {
		logger.Fatal(ctx, "grpc-client-not-created")
	}
	// Start the core grpc client
	go a.coreClient.Start(ctx, setAndTestCoreServiceHandler)

	// Create the open OLT adapter
	if a.oltAdapter, err = a.startOpenOLT(ctx, a.coreClient, a.eventProxy, a.config, cm); err != nil {
		logger.Fatalw(ctx, "error-starting-openolt", log.Fields{"error": err})
	}

	// Create and start the grpc server
	a.grpcServer = vgrpc.NewGrpcServer(a.config.GrpcAddress, nil, false, p)

	//Register the  adapter  service
	a.addAdapterService(ctx, a.grpcServer, a.oltAdapter)

	//Register the olt inter-adapter  service
	a.addOltInterAdapterService(ctx, a.grpcServer, a.oltAdapter)

	// Start the grpc server
	go a.startGRPCService(ctx, a.grpcServer, oltAdapterService)

	// Register this adapter to the Core - retries indefinitely
	if err = a.registerWithCore(ctx, coreService, -1); err != nil {
		logger.Fatal(ctx, "error-registering-with-core")
	}

	// check the readiness and liveliness and update the probe status
	a.checkServicesReadiness(ctx)
}

// TODO:  Any action the adapter needs to do following a Core restart?
func (a *adapter) coreRestarted(ctx context.Context, endPoint string) error {
	logger.Errorw(ctx, "core-restarted", log.Fields{"endpoint": endPoint})
	return nil
}

// setAndTestCoreServiceHandler is used to test whether the remote gRPC service is up
func setAndTestCoreServiceHandler(ctx context.Context, conn *grpc.ClientConn, clientConn *common.Connection) interface{} {
	svc := core_service.NewCoreServiceClient(conn)
	if h, err := svc.GetHealthStatus(ctx, clientConn); err != nil || h.State != health.HealthStatus_HEALTHY {
		return nil
	}
	return svc
}

/**
This function checks the liveliness and readiness of the kakfa and kv-client services
and update the status in the probe.
*/
func (a *adapter) checkServicesReadiness(ctx context.Context) {
	// checks the kafka readiness
	go kafka.MonitorKafkaReadiness(ctx, a.kafkaClient, a.config.LiveProbeInterval, a.config.NotLiveProbeInterval, clusterMessagingService)

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
				probe.UpdateStatusFromContext(ctx, kvService, probe.ServiceStatusNotReady)
				timeout = a.config.NotLiveProbeInterval
			} else {
				// kv-store is reachable , updating the status to running state
				probe.UpdateStatusFromContext(ctx, kvService, probe.ServiceStatusRunning)
				timeout = a.config.LiveProbeInterval / 2
			}
			// Check if the timer has expired or not
			if !timeoutTimer.Stop() {
				<-timeoutTimer.C
			}
		case <-timeoutTimer.C:
			// Check the status of the kv-store. Use timeout of 2 seconds to avoid forever blocking
			logger.Info(ctx, "kv-store liveliness-recheck")
			timeoutCtx, cancelFunc := context.WithTimeout(ctx, 2*time.Second)

			kvStoreChannel <- a.kvClient.IsConnectionUp(timeoutCtx)
			// Cleanup cancel func resources
			cancelFunc()
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
			logger.Infow(ctx, "fail-to-release-all-reservations", log.Fields{"error": err})
		}
		// Close the DB connection
		a.kvClient.Close(ctx)
	}

	if a.eventProxy != nil {
		a.eventProxy.Stop()
	}

	if a.kafkaClient != nil {
		a.kafkaClient.Stop(ctx)
	}

	// Stop core client
	if a.coreClient != nil {
		a.coreClient.Stop(ctx)
	}

	// TODO: Stop child devices connections

	// TODO:  More cleanup
}

func newKVClient(ctx context.Context, storeType, address string, timeout time.Duration) (kvstore.Client, error) {

	logger.Infow(ctx, "kv-store-type", log.Fields{"store": storeType})
	switch storeType {
	case "etcd":
		return kvstore.NewEtcdClient(ctx, address, timeout, log.FatalLevel)
	}
	return nil, errors.New("unsupported-kv-store")
}

func newKafkaClient(ctx context.Context, clientType, address string) (kafka.Client, error) {

	logger.Infow(ctx, "common-client-type", log.Fields{"client": clientType})
	switch clientType {
	case "sarama":
		return kafka.NewSaramaClient(
			kafka.Address(address),
			kafka.ProducerReturnOnErrors(true),
			kafka.ProducerReturnOnSuccess(true),
			kafka.ProducerMaxRetries(6),
			kafka.ProducerRetryBackoff(time.Millisecond*30),
			kafka.MetadatMaxRetries(15)), nil
	}

	return nil, errors.New("unsupported-client-type")
}

func (a *adapter) setKVClient(ctx context.Context) error {
	client, err := newKVClient(ctx, a.config.KVStoreType, a.config.KVStoreAddress, a.config.KVStoreTimeout)
	if err != nil {
		a.kvClient = nil
		return err
	}
	a.kvClient = client

	return nil
}

// startGRPCService creates the grpc service handlers, registers it to the grpc server and starts the server
func (a *adapter) startGRPCService(ctx context.Context, server *vgrpc.GrpcServer, serviceName string) {
	logger.Infow(ctx, "starting-grpc-service", log.Fields{"service": serviceName})

	probe.UpdateStatusFromContext(ctx, serviceName, probe.ServiceStatusRunning)
	logger.Infow(ctx, "grpc-service-started", log.Fields{"service": serviceName})

	server.Start(ctx)
	probe.UpdateStatusFromContext(ctx, serviceName, probe.ServiceStatusStopped)
}

func (a *adapter) addAdapterService(ctx context.Context, server *vgrpc.GrpcServer, handler adapter_service.AdapterServiceServer) {
	logger.Info(ctx, "adding-adapter-service")

	server.AddService(func(gs *grpc.Server) {
		adapter_service.RegisterAdapterServiceServer(gs, handler)
	})
}

func (a *adapter) addOltInterAdapterService(ctx context.Context, server *vgrpc.GrpcServer, handler olt_inter_adapter_service.OltInterAdapterServiceServer) {
	logger.Info(ctx, "adding-olt-inter-adapter-service")

	server.AddService(func(gs *grpc.Server) {
		olt_inter_adapter_service.RegisterOltInterAdapterServiceServer(gs, handler)
	})
}

func (a *adapter) startOpenOLT(ctx context.Context, cc *vgrpc.Client, ep eventif.EventProxy,
	cfg *config.AdapterFlags, cm *conf.ConfigManager) (*ac.OpenOLT, error) {
	logger.Info(ctx, "starting-open-olt")
	var err error
	sOLT := ac.NewOpenOLT(ctx, cc, ep, cfg, cm)

	if err = sOLT.Start(ctx); err != nil {
		return nil, err
	}

	logger.Info(ctx, "open-olt-started")
	return sOLT, nil
}

func (a *adapter) registerWithCore(ctx context.Context, serviceName string, retries int) error {
	adapterID := fmt.Sprintf("openolt_%d", a.config.CurrentReplica)
	logger.Infow(ctx, "registering-with-core", log.Fields{
		"adapterID":      adapterID,
		"currentReplica": a.config.CurrentReplica,
		"totalReplicas":  a.config.TotalReplicas,
	})
	adapterDescription := &voltha.Adapter{
		Id:      adapterID, // Unique name for the device type
		Vendor:  "VOLTHA OpenOLT",
		Version: version.VersionInfo.Version,
		// The Endpoint refers to the address this service is listening on.
		Endpoint:       a.config.AdapterEndpoint,
		Type:           "openolt",
		CurrentReplica: int32(a.config.CurrentReplica),
		TotalReplicas:  int32(a.config.TotalReplicas),
	}
	types := []*voltha.DeviceType{{
		Id:                          "openolt",
		AdapterType:                 "openolt", // Type of the adapter that handles device type
		Adapter:                     "openolt", // Deprecated attribute
		AcceptsBulkFlowUpdate:       false,     // Currently openolt adapter does not support bulk flow handling
		AcceptsAddRemoveFlowUpdates: true}}
	deviceTypes := &voltha.DeviceTypes{Items: types}
	count := 0
	for {
		gClient, err := a.coreClient.GetCoreServiceClient()
		if gClient != nil {
			if _, err = gClient.RegisterAdapter(log.WithSpanFromContext(context.TODO(), ctx), &ca.AdapterRegistration{
				Adapter: adapterDescription,
				DTypes:  deviceTypes}); err == nil {
				break
			}
		}
		logger.Warnw(ctx, "registering-with-core-failed", log.Fields{"endpoint": a.config.CoreEndpoint, "error": err, "count": count, "gclient": gClient})
		if retries == count {
			return err
		}
		count++
		// Take a nap before retrying
		time.Sleep(2 * time.Second)
	}
	probe.UpdateStatusFromContext(ctx, serviceName, probe.ServiceStatusRunning)
	logger.Info(ctx, "registered-with-core")
	return nil
}

func waitForExit(ctx context.Context) int {
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
			logger.Infow(ctx, "closing-signal-received", log.Fields{"signal": s})
			exitChannel <- 0
		default:
			logger.Infow(ctx, "unexpected-signal-received", log.Fields{"signal": s})
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
	ctx := context.Background()
	start := time.Now()

	cf := config.NewAdapterFlags()
	cf.ParseCommandArguments()

	// Setup logging

	logLevel, err := log.StringToLogLevel(cf.LogLevel)
	if err != nil {
		logger.Fatalf(ctx, "Cannot setup logging, %s", err)
	}

	// Setup default logger - applies for packages that do not have specific logger set
	if _, err := log.SetDefaultLogger(log.JSON, logLevel, log.Fields{"instanceId": cf.InstanceID}); err != nil {
		logger.With(log.Fields{"error": err}).Fatal(ctx, "Cannot setup logging")
	}

	// Update all loggers (provisionned via init) with a common field
	if err := log.UpdateAllLoggers(log.Fields{"instanceId": cf.InstanceID}); err != nil {
		logger.With(log.Fields{"error": err}).Fatal(ctx, "Cannot setup logging")
	}

	log.SetAllLogLevel(logLevel)

	realMain()

	defer func() {
		err := log.CleanUp()
		if err != nil {
			logger.Errorw(context.Background(), "unable-to-flush-any-buffered-log-entries", log.Fields{"error": err})
		}
	}()

	// Print version / build information and exit
	if cf.DisplayVersionOnly {
		printVersion()
		return
	}

	// Print banner if specified
	if cf.Banner {
		printBanner()
	}

	logger.Infow(ctx, "config", log.Fields{"config": *cf})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ad := newAdapter(cf)

	p := &probe.Probe{}
	go p.ListenAndServe(ctx, ad.config.ProbeAddress)

	probeCtx := context.WithValue(ctx, probe.ProbeContextKey, p)

	closer, err := log.GetGlobalLFM().InitTracingAndLogCorrelation(cf.TraceEnabled, cf.TraceAgentAddress, cf.LogCorrelationEnabled)
	if err != nil {
		logger.Warnw(ctx, "unable-to-initialize-tracing-and-log-correlation-module", log.Fields{"error": err})
	} else {
		defer log.TerminateTracing(closer)
	}

	go ad.start(probeCtx)

	code := waitForExit(ctx)
	logger.Infow(ctx, "received-a-closing-signal", log.Fields{"code": code})

	// Cleanup before leaving
	ad.stop(ctx)

	elapsed := time.Since(start)
	logger.Infow(ctx, "run-time", log.Fields{"instanceId": ad.config.InstanceID, "time": elapsed / time.Second})
}
