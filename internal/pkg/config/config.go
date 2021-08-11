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
	"os"
	"time"
)

// Open OLT default constants
const (
	EtcdStoreName               = "etcd"
	defaultInstanceid           = "openOlt001"
	defaultKafkaclusteraddress  = "127.0.0.1:9092"
	defaultKvstoretype          = EtcdStoreName
	defaultKvstoretimeout       = 5 * time.Second
	defaultRPCTimeout           = 10 * time.Second
	defaultKvstoreaddress       = "127.0.0.1:2379" // Port: Consul = 8500; Etcd = 2379
	defaultLoglevel             = "WARN"
	defaultBanner               = false
	defaultDisplayVersionOnly   = false
	defaultEventtopic           = "voltha.events"
	defaultOnunumber            = 1
	defaultProbeAddress         = ":8080"
	defaultLiveProbeInterval    = 60 * time.Second
	defaultNotLiveProbeInterval = 5 * time.Second // Probe more frequently when not alive
	//defaultHeartbeatCheckInterval is the time in seconds the adapter will keep checking the hardware for heartbeat.
	defaultHeartbeatCheckInterval = 15 * time.Second
	// defaultHeartbeatFailReportInterval is the time adapter will wait before updating the state to the core.
	defaultHeartbeatFailReportInterval = 0 * time.Second
	defaultGrpcAddress                 = ":50060"
	defaultCoreEndpoint                = ":55555"
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
	defaultMinBackoffRetryDelay  = 500 * time.Millisecond
	defaultMaxBackoffRetryDelay  = 10 * time.Second
	defaultAdapterEndpoint       = "adapter-open-olt"
)

// AdapterFlags represents the set of configurations used by the read-write adaptercore service
type AdapterFlags struct {
	// Command line parameters
	AdapterName                 string
	InstanceID                  string // NOTE what am I used for? why not cli but only ENV? TODO expose in the chart
	KafkaClusterAddress         string
	KVStoreType                 string
	KVStoreTimeout              time.Duration
	KVStoreAddress              string
	RPCTimeout                  time.Duration
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
	GrpcAddress                 string
	CoreEndpoint                string
	CurrentReplica              int
	TotalReplicas               int
	TraceEnabled                bool
	TraceAgentAddress           string
	LogCorrelationEnabled       bool
	OmccEncryption              bool
	EnableONUStats              bool
	EnableGEMStats              bool
	MinBackoffRetryDelay        time.Duration
	MaxBackoffRetryDelay        time.Duration
	AdapterEndpoint             string
}

// NewAdapterFlags returns a new RWCore config
func NewAdapterFlags() *AdapterFlags {
	var adapterFlags = AdapterFlags{ // Default values
		InstanceID:                  defaultInstanceid,
		KafkaClusterAddress:         defaultKafkaclusteraddress,
		KVStoreType:                 defaultKvstoretype,
		KVStoreTimeout:              defaultKvstoretimeout,
		KVStoreAddress:              defaultKvstoreaddress,
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
		GrpcAddress:                 defaultGrpcAddress,
		CoreEndpoint:                defaultCoreEndpoint,
		GrpcTimeoutInterval:         defaultGrpcTimeoutInterval,
		TraceEnabled:                defaultTraceEnabled,
		TraceAgentAddress:           defaultTraceAgentAddress,
		LogCorrelationEnabled:       defaultLogCorrelationEnabled,
		OmccEncryption:              defaultOmccEncryption,
		EnableONUStats:              defaultEnableONUStats,
		EnableGEMStats:              defaultEnableGEMStats,
		RPCTimeout:                  defaultRPCTimeout,
		MinBackoffRetryDelay:        defaultMinBackoffRetryDelay,
		MaxBackoffRetryDelay:        defaultMaxBackoffRetryDelay,
	}
	return &adapterFlags
}

// ParseCommandArguments parses the arguments when running read-write adaptercore service
func (so *AdapterFlags) ParseCommandArguments() {

	flag.StringVar(&(so.KafkaClusterAddress),
		"kafka_cluster_address",
		defaultKafkaclusteraddress,
		"Kafka - Cluster messaging address")

	flag.StringVar(&(so.EventTopic),
		"event_topic",
		defaultEventtopic,
		"Event topic")

	flag.StringVar(&(so.KVStoreType),
		"kv_store_type",
		defaultKvstoretype,
		"KV store type")

	flag.DurationVar(&(so.KVStoreTimeout),
		"kv_store_request_timeout",
		defaultKvstoretimeout,
		"The default timeout when making a kv store request")

	flag.StringVar(&(so.KVStoreAddress),
		"kv_store_address",
		defaultKvstoreaddress,
		"KV store address")

	flag.StringVar(&(so.LogLevel),
		"log_level",
		defaultLoglevel,
		"Log level")

	flag.IntVar(&(so.OnuNumber),
		"onu_number",
		defaultOnunumber,
		"Number of ONUs")

	flag.BoolVar(&(so.Banner),
		"banner",
		defaultBanner,
		"Show startup banner log lines")

	flag.BoolVar(&(so.DisplayVersionOnly),
		"version",
		defaultDisplayVersionOnly,
		"Show version information and exit")

	flag.StringVar(&(so.ProbeAddress),
		"probe_address",
		defaultProbeAddress,
		"The address on which to listen to answer liveness and readiness probe queries over HTTP.")

	flag.DurationVar(&(so.LiveProbeInterval),
		"live_probe_interval",
		defaultLiveProbeInterval,
		"Number of seconds for the default liveliness check")

	flag.DurationVar(&(so.NotLiveProbeInterval),
		"not_live_probe_interval",
		defaultNotLiveProbeInterval,
		"Number of seconds for liveliness check if probe is not running")

	flag.DurationVar(&(so.HeartbeatCheckInterval),
		"heartbeat_check_interval",
		defaultHeartbeatCheckInterval,
		"Number of seconds for heartbeat check interval")

	flag.DurationVar(&(so.HeartbeatFailReportInterval),
		"heartbeat_fail_interval",
		defaultHeartbeatFailReportInterval,
		"Number of seconds adapter has to wait before reporting core on the heartbeat check failure")

	flag.DurationVar(&(so.GrpcTimeoutInterval),
		"grpc_timeout_interval",
		defaultGrpcTimeoutInterval,
		"Number of seconds for GRPC timeout")

	flag.IntVar(&(so.CurrentReplica),
		"current_replica",
		defaultCurrentReplica,
		"Replica number of this particular instance")

	flag.IntVar(&(so.TotalReplicas),
		"total_replica",
		defaultTotalReplicas,
		"Total number of instances for this adapter")

	flag.BoolVar(&(so.TraceEnabled),
		"trace_enabled",
		defaultTraceEnabled,
		"Whether to send logs to tracing agent?")

	flag.StringVar(&(so.TraceAgentAddress),
		"trace_agent_address",
		defaultTraceAgentAddress,
		"The address of tracing agent to which span info should be sent")

	flag.BoolVar(&(so.LogCorrelationEnabled),
		"log_correlation_enabled",
		defaultLogCorrelationEnabled,
		"Whether to enrich log statements with fields denoting operation being executed for achieving correlation?")

	flag.BoolVar(&(so.OmccEncryption),
		"omcc_encryption",
		defaultOmccEncryption,
		"OMCI Channel encryption status")

	flag.BoolVar(&(so.EnableONUStats),
		"enable_onu_stats",
		defaultEnableONUStats,
		"Enable ONU Statistics")

	flag.BoolVar(&(so.EnableGEMStats),
		"enable_gem_stats",
		defaultEnableGEMStats,
		"Enable GEM Statistics")

	flag.StringVar(&(so.GrpcAddress),
		"grpc_address",
		defaultGrpcAddress,
		"Adapter GRPC Server address")

	flag.StringVar(&(so.CoreEndpoint),
		"core_endpoint",
		defaultCoreEndpoint,
		"Core endpoint")

	flag.StringVar(&(so.AdapterEndpoint),
		"adapter_endpoint",
		defaultAdapterEndpoint,
		"Adapter Endpoint")

	flag.DurationVar(&(so.RPCTimeout),
		"rpc_timeout",
		defaultRPCTimeout,
		"The default timeout when making an RPC request")

	flag.DurationVar(&(so.MinBackoffRetryDelay),
		"min_retry_delay",
		defaultMinBackoffRetryDelay,
		"The minimum number of milliseconds to delay before a connection retry attempt")

	flag.DurationVar(&(so.MaxBackoffRetryDelay),
		"max_retry_delay",
		defaultMaxBackoffRetryDelay,
		"The maximum number of milliseconds to delay before a connection retry attempt")

	flag.Parse()
	containerName := getContainerInfo()
	if len(containerName) > 0 {
		so.InstanceID = containerName
	}

}

func getContainerInfo() string {
	return os.Getenv("HOSTNAME")
}
