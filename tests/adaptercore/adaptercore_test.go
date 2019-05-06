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
package adaptercore

import (
	com "github.com/opencord/voltha-go/adapters/common"
	"github.com/opencord/voltha-go/kafka"
	ac "github.com/opencord/voltha-openolt-adapter/adaptercore"
	"testing"
	"time"
)

type TestOpenOLT struct {
	OpenOltDevice *ac.OpenOLT
	coreProxy     *com.CoreProxy
	adapterProxy  *com.AdapterProxy
	alarmProxy    *com.AlarmProxy
	kafkaClient   kafka.Client
	kafkaICProxy  *kafka.InterContainerProxy
	numOnus       int
	KVStoreHost   string
	KVStorePort   int
	KVStoreType   string
}

var td TestOpenOLT

func setup() {
	td.kafkaClient = kafka.NewSaramaClient(
		kafka.Host("127.0.0.1"),
		kafka.Port(9092),
		kafka.ProducerReturnOnErrors(true),
		kafka.ProducerReturnOnSuccess(true),
		kafka.ProducerMaxRetries(6),
		kafka.ProducerRetryBackoff(time.Millisecond*30))
	td.kafkaICProxy, _ = kafka.NewInterContainerProxy(
		kafka.InterContainerHost("127.0.0.1"),
		kafka.InterContainerPort(9092),
		kafka.MsgClient(td.kafkaClient),
		kafka.DefaultTopic(&kafka.Topic{Name: "openolt"}))
	td.kafkaICProxy.Start()
	td.numOnus = 16
	td.KVStoreHost = "localhost"
	td.KVStorePort = 8500
	td.KVStoreType = "Consul"
	td.adapterProxy = com.NewAdapterProxy(td.kafkaICProxy, "openolt", "rw-core")
	td.alarmProxy = com.NewAlarmProxy(com.MsgClient(td.kafkaClient),
		com.MsgTopic(kafka.Topic{Name: "voltha.alarms"}))
	td.coreProxy = com.NewCoreProxy(td.kafkaICProxy, "openolt", "rw-core")
	td.OpenOltDevice = ac.NewOpenOLT(nil, td.kafkaICProxy, td.coreProxy,
		td.adapterProxy, td.alarmProxy,
		td.numOnus, td.KVStoreHost,
		td.KVStorePort, td.KVStoreType)
}

func shutdown() {
	td.kafkaICProxy.Stop()
	td.OpenOltDevice = nil
	td.adapterProxy = nil
	td.alarmProxy = nil
	td.coreProxy = nil
	td.kafkaICProxy = nil
}

/* TODO: Implemention of Test Cases for the adaptercore

   Empty test cases has been added to make sure the
   CI build does not fail. These test cases will be
   added at a later stage
*/
func TestCreateDeviceTopic(t *testing.T) {}

func TestAdopt_device(t *testing.T) {}

func TestGet_ofp_device_info(t *testing.T) {}

func TestGet_ofp_port_info(t *testing.T) {}

func TestDisable_device(t *testing.T) {}

func TestReenable_device(t *testing.T) {}
