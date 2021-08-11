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

//Package mocks provides the mocks for openolt-adapter.
package mocks

import (
	"context"
	"errors"

	"github.com/opencord/voltha-protos/v5/go/voltha"
)

// MockEventProxy for mocking EventProxyIntf
type MockEventProxy struct {
}

// SendDeviceEvent mocks the SendDeviceEvent function
func (me *MockEventProxy) SendDeviceEvent(ctx context.Context, deviceEvent *voltha.DeviceEvent, category voltha.EventCategory_Types,
	subCategory voltha.EventSubCategory_Types, raisedTs int64) error {
	if raisedTs == 0 {
		return errors.New("raisedTS cannot be zero")
	}
	return nil
}

// SendKpiEvent mocks the SendKpiEvent function
func (me *MockEventProxy) SendKpiEvent(ctx context.Context, id string, deviceEvent *voltha.KpiEvent2, category voltha.EventCategory_Types,
	subCategory voltha.EventSubCategory_Types, raisedTs int64) error {
	if raisedTs == 0 {
		return errors.New("raisedTS cannot be zero")
	}
	return nil
}

// SendRPCEvent mocks the SendRPCEvent function
func (me *MockEventProxy) SendRPCEvent(ctx context.Context, id string, deviceEvent *voltha.RPCEvent, category voltha.EventCategory_Types,
	subCategory *voltha.EventSubCategory_Types, raisedTs int64) error {
	if raisedTs == 0 {
		return errors.New("raisedTS cannot be zero")
	}
	return nil
}

// EnableLivenessChannel mocks the EnableLivenessChannel function
func (me *MockEventProxy) EnableLivenessChannel(ctx context.Context, enable bool) chan bool {
	return nil
}

// SendLiveness mocks the SendLiveness function
func (me *MockEventProxy) SendLiveness(ctx context.Context) error {
	return nil
}

// Start starts the proxy
func (me *MockEventProxy) Start() error {
	return nil
}

// Stop stops the proxy
func (me *MockEventProxy) Stop() {
}
