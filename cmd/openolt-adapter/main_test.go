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
	"testing"

	conf "github.com/opencord/voltha-lib-go/v7/pkg/config"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"
	mgrpc "github.com/opencord/voltha-lib-go/v7/pkg/mocks/grpc"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"go.etcd.io/etcd/pkg/mock/mockserver"
)

func newMockAdapter() *adapter {
	cf := config.NewAdapterFlags()
	cf.KVStoreType = "etcd"
	ad := newAdapter(cf)
	return ad
}
func Test_adapter_setKVClient(t *testing.T) {
	adapt := newMockAdapter()
	adapt1 := newMockAdapter()
	adapt1.config.KVStoreType = "etcd"
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
	// Create a and start a mock Core GRPC server
	ms, err := mgrpc.NewMockGRPCServer(ctx)
	if err != nil {
		t.Errorf("grpc server: expected error:nil, got error: %v", err)
	}
	ms.AddCoreService(ctx, &vgrpc.MockCoreServiceHandler{})
	go ms.Start(ctx)
	defer ms.Stop()

	if ad.coreClient, err = vgrpc.NewClient(
		"olt-endpoint",
		ms.ApiEndpoint,
		ad.coreRestarted); err != nil {
		t.Errorf("grpc client: expected error:nil, got error: %v", err)
	}
	// Start the core grpc client
	go ad.coreClient.Start(ctx, setAndTestCoreServiceHandler)
	defer ad.coreClient.Stop(ctx)
	err = ad.registerWithCore(ctx, coreService, 1)
	if err != nil {
		t.Errorf("Expected error:nil, got error: %v", err)
	}
}

func Test_startOpenOLT(t *testing.T) {
	a, _ := mockserver.StartMockServers(1)
	cm := &conf.ConfigManager{}
	_ = a.StartAt(0)
	defer a.StopAt(0)

	ad := newMockAdapter()
	oolt, err := ad.startOpenOLT(context.TODO(), nil, ad.eventProxy, ad.config, cm)
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
		{"newKafkaClient", args{clientType: "sarama", address: adapter.config.KafkaClusterAddress}, false},
		{"newKafkaClient", args{clientType: "sarama", address: adapter.config.KafkaClusterAddress}, false},
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
