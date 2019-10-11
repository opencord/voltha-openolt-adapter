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
	"errors"

	"github.com/opencord/voltha-protos/go/voltha"
)

// MockEventProxy for mocking EventProxyIntf
type MockEventProxy struct {
}

// SendDeviceEvent mocks the SendDeviceEvent function
func (me *MockEventProxy) SendDeviceEvent(deviceEvent *voltha.DeviceEvent, category voltha.EventCategory_EventCategory,
	subCategory voltha.EventSubCategory_EventSubCategory, raisedTs int64) error {
	if raisedTs == 0 {
		return errors.New("raisedTS cannot be zero")
	}
	return nil
}

func (me *MockEventProxy) SendKpiEvent(deviceEvent *voltha.KpiEvent2, category voltha.EventCategory_EventCategory,
	subCategory voltha.EventSubCategory_EventSubCategory, raisedTs int64) error {
	if raisedTs == 0 {
		return errors.New("raisedTS cannot be zero")
	}
	return nil
}
