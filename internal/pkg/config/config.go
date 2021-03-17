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

//Package config provides the Log, kvstore, Kafka configuration
package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Open OLT default constants
const (
	EtcdStoreName               = "etcd"
	defaultInstanceid           = "openOlt001"
	defaultKafkaadapteraddress  = "127.0.0.1:9092"
	defaultKafkaclusteraddress  = "127.0.0.1:9094"
	defaultKvstoretype          = EtcdStoreName
	defaultKvstoretimeout       = 5 * time.Second
	defaultKvstoreaddress       = "127.0.0.1:2379" // Port: Consul = 8500; Etcd = 2379
	defaultLoglevel             = "WARN"
	defaultBanner               = false
	defaultDisplayVersionOnly   = false
	defaultTopic                = "openolt"
	defaultCoretopic            = "rwcore"
	defaultEventtopic           = "voltha.events"
	defaultOnunumber            = 1
	defaultProbeAddress         = ":8080"
	defaultLiveProbeInterval    = 60 * time.Second
	defaultNotLiveProbeInterval = 5 * time.Second // Probe more frequently when not alive
	//defaultHeartbeatCheckInterval is the time in seconds the adapter will keep checking the hardware for heartbeat.
	defaultHeartbeatCheckInterval = 15 * time.Second
	// defaultHeartbeatFailReportInterval is the time adapter will wait before updating the state to the core.
	defaultHeartbeatFailReportInterval = 0 * time.Second
	//defaultGrpcTimeoutInterval is the time in seconds a grpc call will wait before returning error.
	defaultGrpcTimeoutInterval   = 2 * time.Second
	defaultCurrentReplica        = 1
	defaultTotalReplicas         = 1
	defaultTraceEnabled          = false
	defaultTraceAgentAddress     = "127.0.0.1:6831"
	defaultLogCorrelationEnabled = true
	defaultOmccEncryption        = false
	defaultEnableONUStats        = false
	defaultEnableGEMStats        = false
)

// AdapterFlags represents the set of configurations used by the read-write adaptercore service
type AdapterFlags struct {
	// Command line parameters
	AdapterName                 string
	InstanceID                  string // NOTE what am I used for? why not cli but only ENV? TODO expose in the chart
	KafkaAdapterAddress         string
	KafkaClusterAddress         string
	KVStoreType                 string
	KVStoreTimeout              time.Duration
	KVStoreAddress              string
	Topic                       string
	CoreTopic                   string
	EventTopic                  string
	LogLevel                    string
	OnuNumber                   int
	Banner                      bool
	DisplayVersionOnly          bool
	ProbeAddress                string
	LiveProbeInterval           time.Duration
	NotLiveProbeInterval        time.Duration
	HeartbeatCheckInterval      time.Duration
	HeartbeatFailReportInterval time.Duration
	GrpcTimeoutInterval         time.Duration
	CurrentReplica              int
	TotalReplicas               int
	TraceEnabled                bool
	TraceAgentAddress           string
	LogCorrelationEnabled       bool
	OmccEncryption              bool
	EnableONUStats              bool
	EnableGEMStats              bool
}

// NewAdapterFlags returns a new RWCore config
func NewAdapterFlags() *AdapterFlags {
	var adapterFlags = AdapterFlags{ // Default values
		InstanceID:                  defaultInstanceid,
		KafkaAdapterAddress:         defaultKafkaadapteraddress,
		KafkaClusterAddress:         defaultKafkaclusteraddress,
		KVStoreType:                 defaultKvstoretype,
		KVStoreTimeout:              defaultKvstoretimeout,
		KVStoreAddress:              defaultKvstoreaddress,
		Topic:                       defaultTopic,
		CoreTopic:                   defaultCoretopic,
		EventTopic:                  defaultEventtopic,
		LogLevel:                    defaultLoglevel,
		OnuNumber:                   defaultOnunumber,
		Banner:                      defaultBanner,
		DisplayVersionOnly:          defaultDisplayVersionOnly,
		ProbeAddress:                defaultProbeAddress,
		LiveProbeInterval:           defaultLiveProbeInterval,
		NotLiveProbeInterval:        defaultNotLiveProbeInterval,
		HeartbeatCheckInterval:      defaultHeartbeatCheckInterval,
		HeartbeatFailReportInterval: defaultHeartbeatFailReportInterval,
		GrpcTimeoutInterval:         defaultGrpcTimeoutInterval,
		TraceEnabled:                defaultTraceEnabled,
		TraceAgentAddress:           defaultTraceAgentAddress,
		LogCorrelationEnabled:       defaultLogCorrelationEnabled,
		OmccEncryption:              defaultOmccEncryption,
		EnableONUStats:              defaultEnableONUStats,
		EnableGEMStats:              defaultEnableGEMStats,
	}
	return &adapterFlags
}

// ParseCommandArguments parses the arguments when running read-write adaptercore service
func (so *AdapterFlags) ParseCommandArguments() {

	help := fmt.Sprintf("Kafka - Adapter messaging address")
	flag.StringVar(&(so.KafkaAdapterAddress), "kafka_adapter_address", defaultKafkaadapteraddress, help)

	help = fmt.Sprintf("Kafka - Cluster messaging address")
	flag.StringVar(&(so.KafkaClusterAddress), "kafka_cluster_address", defaultKafkaclusteraddress, help)

	help = fmt.Sprintf("Open OLT topic")
	flag.StringVar(&(so.Topic), "adapter_topic", defaultTopic, help)

	help = fmt.Sprintf("Core topic")
	flag.StringVar(&(so.CoreTopic), "core_topic", defaultCoretopic, help)

	help = fmt.Sprintf("Event topic")
	flag.StringVar(&(so.EventTopic), "event_topic", defaultEventtopic, help)

	help = fmt.Sprintf("KV store type")
	flag.StringVar(&(so.KVStoreType), "kv_store_type", defaultKvstoretype, help)

	help = fmt.Sprintf("The default timeout when making a kv store request")
	flag.DurationVar(&(so.KVStoreTimeout), "kv_store_request_timeout", defaultKvstoretimeout, help)

	help = fmt.Sprintf("KV store address")
	flag.StringVar(&(so.KVStoreAddress), "kv_store_address", defaultKvstoreaddress, help)

	help = fmt.Sprintf("Log level")
	flag.StringVar(&(so.LogLevel), "log_level", defaultLoglevel, help)

	help = fmt.Sprintf("Number of ONUs")
	flag.IntVar(&(so.OnuNumber), "onu_number", defaultOnunumber, help)

	help = fmt.Sprintf("Show startup banner log lines")
	flag.BoolVar(&(so.Banner), "banner", defaultBanner, help)

	help = fmt.Sprintf("Show version information and exit")
	flag.BoolVar(&(so.DisplayVersionOnly), "version", defaultDisplayVersionOnly, help)

	help = fmt.Sprintf("The address on which to listen to answer liveness and readiness probe queries over HTTP.")
	flag.StringVar(&(so.ProbeAddress), "probe_address", defaultProbeAddress, help)

	help = fmt.Sprintf("Number of seconds for the default liveliness check")
	flag.DurationVar(&(so.LiveProbeInterval), "live_probe_interval", defaultLiveProbeInterval, help)

	help = fmt.Sprintf("Number of seconds for liveliness check if probe is not running")
	flag.DurationVar(&(so.NotLiveProbeInterval), "not_live_probe_interval", defaultNotLiveProbeInterval, help)

	help = fmt.Sprintf("Number of seconds for heartbeat check interval.")
	flag.DurationVar(&(so.HeartbeatCheckInterval), "heartbeat_check_interval", defaultHeartbeatCheckInterval, help)

	help = fmt.Sprintf("Number of seconds adapter has to wait before reporting core on the hearbeat check failure.")
	flag.DurationVar(&(so.HeartbeatFailReportInterval), "heartbeat_fail_interval", defaultHeartbeatFailReportInterval, help)

	help = fmt.Sprintf("Number of seconds for GRPC timeout.")
	flag.DurationVar(&(so.GrpcTimeoutInterval), "grpc_timeout_interval", defaultGrpcTimeoutInterval, help)

	help = "Replica number of this particular instance (default: %s)"
	flag.IntVar(&(so.CurrentReplica), "current_replica", defaultCurrentReplica, help)

	help = "Total number of instances for this adapter"
	flag.IntVar(&(so.TotalReplicas), "total_replica", defaultTotalReplicas, help)

	help = fmt.Sprintf("Whether to send logs to tracing agent?")
	flag.BoolVar(&(so.TraceEnabled), "trace_enabled", defaultTraceEnabled, help)

	help = fmt.Sprintf("The address of tracing agent to which span info should be sent.")
	flag.StringVar(&(so.TraceAgentAddress), "trace_agent_address", defaultTraceAgentAddress, help)

	help = fmt.Sprintf("Whether to enrich log statements with fields denoting operation being executed for achieving correlation?")
	flag.BoolVar(&(so.LogCorrelationEnabled), "log_correlation_enabled", defaultLogCorrelationEnabled, help)

	help = fmt.Sprintf("OMCI Channel encryption status")
	flag.BoolVar(&(so.OmccEncryption), "omcc_encryption", defaultOmccEncryption, help)

	help = fmt.Sprintf("Enable ONU Statistics")
	flag.BoolVar(&(so.EnableONUStats), "enable_onu_stats", defaultEnableONUStats, help)

	help = fmt.Sprintf("Enable GEM Statistics")
	flag.BoolVar(&(so.EnableGEMStats), "enable_gem_stats", defaultEnableGEMStats, help)

	flag.Parse()
	containerName := getContainerInfo()
	if len(containerName) > 0 {
		so.InstanceID = containerName
	}

}

func getContainerInfo() string {
	return os.Getenv("HOSTNAME")
}
