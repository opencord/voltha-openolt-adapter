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
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opencord/voltha-lib-go/v3/pkg/kafka"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/pkg/mocks"
	ca "github.com/opencord/voltha-protos/v3/go/inter_container"
	"go.etcd.io/etcd/pkg/mock/mockserver"
)

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
	_ = a.StartAt(0)
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
			if err := tt.adapter.setKVClient(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("adapter.setKVClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_adapter_KVClient(t *testing.T) {
	adapt := newMockAdapter()
	a, _ := mockserver.StartMockServers(1)
	_ = a.StartAt(0)
	defer a.StopAt(0)

	if err := adapt.setKVClient(context.Background()); err != nil {
		t.Errorf("adapter.setKVClient() error = %v", err)
	}
}

func Test_registerWithCore(t *testing.T) {
	ad := newMockAdapter()
	ctx := context.TODO()
	err := ad.registerWithCore(ctx, 1)
	if err != nil {
		t.Errorf("Expected error:nil, got error: %v", err)
	}
}
func Test_startInterContainerProxy(t *testing.T) {
	ad := newMockAdapter()
	kc := &mockKafkaClient{}
	ad.kafkaClient = kc
	ctx := context.TODO()
	icp, err := ad.startInterContainerProxy(ctx, 1)
	if icp != nil {
		t.Log("Intercontainer proxy ", icp)
	}
	if err != nil {
		t.Errorf("err %v", err)
	}
}

func Test_startOpenOLT(t *testing.T) {
	a, _ := mockserver.StartMockServers(1)
	_ = a.StartAt(0)
	defer a.StopAt(0)

	ad := newMockAdapter()
	oolt, err := ad.startOpenOLT(context.TODO(), nil,
		ad.coreProxy, ad.adapterProxy, ad.eventProxy, ad.config)
	if oolt != nil {
		t.Log("Open OLT ", oolt)
	}
	if err != nil {
		t.Errorf("err %v", err)
	}
}

func Test_newKafkaClient(t *testing.T) {
	a, _ := mockserver.StartMockServers(1)
	_ = a.StartAt(0)
	defer a.StopAt(0)
	adapter := newMockAdapter()
	type args struct {
		clientType string
		address    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{"newKafkaClient", args{clientType: "sarama", address: adapter.config.KafkaAdapterAddress}, false},
		{"newKafkaClient", args{clientType: "sarama", address: adapter.config.KafkaAdapterAddress}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newKafkaClient(context.Background(), tt.args.clientType, tt.args.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("newKafkaClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

func Test_adapter_setupRequestHandler(t *testing.T) {

	ad := newMockAdapter()

	kip := kafka.NewInterContainerProxy(
		kafka.InterContainerAddress(ad.config.KafkaAdapterAddress),
		kafka.MsgClient(&mockKafkaClient{}),
		kafka.DefaultTopic(&kafka.Topic{Name: ad.config.Topic}))

	ad.kip = kip
	_ = ad.kip.Start(context.Background())

	oolt, _ := ad.startOpenOLT(context.TODO(), nil,
		ad.coreProxy, ad.adapterProxy, ad.eventProxy, ad.config)
	printBanner()
	printVersion()
	ctx := context.TODO()
	if err := ad.setupRequestHandler(ctx, ad.config.InstanceID, oolt); err != nil {
		t.Logf("adapter.setupRequestHandler() error = %v", err)
	}

}

// Kafka client mocker
type mockKafkaClient struct {
}

func (kc *mockKafkaClient) Start(ctx context.Context) error {
	return nil
}
func (kc *mockKafkaClient) Stop(ctx context.Context) {
}
func (kc *mockKafkaClient) CreateTopic(ctx context.Context, topic *kafka.Topic, numPartition int, repFactor int) error {
	if topic != nil {
		return nil
	}
	return errors.New("invalid Topic")
}
func (kc *mockKafkaClient) DeleteTopic(ctx context.Context, topic *kafka.Topic) error {
	if topic != nil {
		return nil
	}
	return errors.New("invalid Topic")
}
func (kc *mockKafkaClient) Subscribe(ctx context.Context, topic *kafka.Topic, kvArgs ...*kafka.KVArg) (<-chan *ca.InterContainerMessage, error) {
	if topic != nil {
		ch := make(chan *ca.InterContainerMessage)
		return ch, nil
	}
	return nil, errors.New("invalid Topic")
}
func (kc *mockKafkaClient) UnSubscribe(ctx context.Context, topic *kafka.Topic, ch <-chan *ca.InterContainerMessage) error {
	if topic == nil {
		return nil
	}
	return errors.New("invalid Topic")
}
func (kc *mockKafkaClient) Send(ctx context.Context, msg interface{}, topic *kafka.Topic, keys ...string) error {
	if topic != nil {
		return nil
	}
	return errors.New("invalid topic")
}

func (kc *mockKafkaClient) SendLiveness(ctx context.Context) error {
	return status.Error(codes.Unimplemented, "SendLiveness")
}

func (kc *mockKafkaClient) EnableLivenessChannel(ctx context.Context, enable bool) chan bool {
	return nil
}

func (kc *mockKafkaClient) EnableHealthinessChannel(ctx context.Context, enable bool) chan bool {
	return nil
}

func (kc *mockKafkaClient) SubscribeForMetadata(context.Context, func(fromTopic string, timestamp time.Time)) {
}
