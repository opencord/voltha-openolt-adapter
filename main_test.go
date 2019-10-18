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
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/opencord/voltha-lib-go/pkg/kafka"
	"github.com/opencord/voltha-lib-go/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/config"
	"github.com/opencord/voltha-openolt-adapter/mocks"
	ca "github.com/opencord/voltha-protos/go/inter_container"
	"go.etcd.io/etcd/pkg/mock/mockserver"
)

func init() {
	log.SetDefaultLogger(log.JSON, log.DebugLevel, nil)
}

func newMockAdapter() *adapter {
	conf := config.NewAdapterFlags()
	conf.KVStoreType = "etcd"
	cp := mocks.MockCoreProxy{}
	ap := mocks.MockAdapterProxy{}
	ad := newAdapter(conf)
	ad.coreProxy = &cp
	ad.adapterProxy = &ap
	return ad
}
func Test_adapter_setKVClient(t *testing.T) {
	adapt := newMockAdapter()
	adapt1 := newMockAdapter()
	adapt1.config.KVStoreType = "consul"
	adapt2 := newMockAdapter()
	adapt2.config.KVStoreType = ""
	a, _ := mockserver.StartMockServers(1)
	a.StartAt(0)
	defer a.StopAt(0)
	tests := []struct {
		name       string
		clienttype string
		adapter    *adapter
		wantErr    bool
	}{
		{"setKVClient", adapt.config.KVStoreType, adapt, false},
		{"setKVClient", adapt1.config.KVStoreType, adapt1, false},
		{"setKVClient", adapt2.config.KVStoreType, adapt2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.adapter.setKVClient(); (err != nil) != tt.wantErr {
				t.Errorf("adapter.setKVClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_adapter_KVClient(t *testing.T) {
	adapt := newMockAdapter()
	a, _ := mockserver.StartMockServers(1)
	a.StartAt(0)
	defer a.StopAt(0)

	if err := adapt.setKVClient(); err != nil {
		t.Errorf("adapter.setKVClient() error = %v", err)
	}
}

func Test_registerWithCore(t *testing.T) {
	ad := newMockAdapter()
	err := ad.registerWithCore(1)
	if err != nil {
		t.Errorf("Expected error:nil, got error: %v", err)
	}
}
func Test_startInterContainerProxy(t *testing.T) {
	ad := newMockAdapter()
	kc := &mockKafkaClient{}
	ad.kafkaClient = kc
	icp, err := ad.startInterContainerProxy(1)
	if icp != nil {
		t.Log("Intercontainer proxy ", icp)
	}
	if err != nil {
		t.Errorf("err %v", err)
	}
}

func Test_startOpenOLT(t *testing.T) {
	a, _ := mockserver.StartMockServers(1)
	a.StartAt(0)
	defer a.StopAt(0)

	ad := newMockAdapter()
	oolt, err := ad.startOpenOLT(context.TODO(), nil,
		ad.coreProxy, ad.adapterProxy, ad.eventProxy, 1, ad.config.KVStoreHost,
		ad.config.KVStorePort, ad.config.KVStoreType)
	if oolt != nil {
		t.Log("Open OLT ", oolt)
	}
	if err != nil {
		t.Errorf("err %v", err)
	}
}

func Test_newKafkaClient(t *testing.T) {
	a, _ := mockserver.StartMockServers(1)
	a.StartAt(0)
	defer a.StopAt(0)
	adapter := newMockAdapter()
	type args struct {
		clientType string
		host       string
		port       int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"newKafkaClient", args{clientType: "sarama", host: adapter.config.KafkaAdapterHost, port: adapter.config.KafkaAdapterPort}, false},
		{"newKafkaClient", args{clientType: "sarama", host: adapter.config.KafkaAdapterHost, port: adapter.config.KafkaAdapterPort}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newKafkaClient(tt.args.clientType, tt.args.host, tt.args.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("newKafkaClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

func Test_adapter_setupRequestHandler(t *testing.T) {

	ad := newMockAdapter()

	kip, _ := kafka.NewInterContainerProxy(
		kafka.InterContainerHost(ad.config.KafkaAdapterHost),
		kafka.InterContainerPort(ad.config.KafkaAdapterPort),
		kafka.MsgClient(&mockKafkaClient{}),
		kafka.DefaultTopic(&kafka.Topic{Name: ad.config.Topic}))

	ad.kip = kip
	ad.kip.Start()

	oolt, _ := ad.startOpenOLT(context.TODO(), nil,
		ad.coreProxy, ad.adapterProxy, ad.eventProxy, 1, ad.config.KVStoreHost,
		ad.config.KVStorePort, ad.config.KVStoreType)
	printBanner()
	printVersion()
	if err := ad.setupRequestHandler(ad.config.InstanceID, oolt); err != nil {
		t.Logf("adapter.setupRequestHandler() error = %v", err)
	}

}

// Kafka client mocker
type mockKafkaClient struct {
}

func (kc *mockKafkaClient) Start() error {
	return nil
}
func (kc *mockKafkaClient) Stop() {
}
func (kc *mockKafkaClient) CreateTopic(topic *kafka.Topic, numPartition int, repFactor int) error {
	if topic != nil {
		return nil
	}
	return errors.New("invalid Topic")
}
func (kc *mockKafkaClient) DeleteTopic(topic *kafka.Topic) error {
	if topic != nil {
		return nil
	}
	return errors.New("invalid Topic")
}
func (kc *mockKafkaClient) Subscribe(topic *kafka.Topic, kvArgs ...*kafka.KVArg) (<-chan *ca.InterContainerMessage, error) {
	if topic != nil {
		ch := make(chan *ca.InterContainerMessage)
		return ch, nil
	}
	return nil, errors.New("invalid Topic")
}
func (kc *mockKafkaClient) UnSubscribe(topic *kafka.Topic, ch <-chan *ca.InterContainerMessage) error {
	if topic == nil {
		return nil
	}
	return errors.New("invalid Topic")
}
func (kc *mockKafkaClient) Send(msg interface{}, topic *kafka.Topic, keys ...string) error {
	if topic != nil {
		return nil
	}
	return errors.New("invalid topic")
}
