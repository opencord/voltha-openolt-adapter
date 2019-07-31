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

	"github.com/golang/protobuf/proto"
	"github.com/opencord/voltha-protos/go/inter_container"
)

// MockAdapterProxy mocks the AdapterProxy interface.
type MockAdapterProxy struct {
}

// SendInterAdapterMessage mocks SendInterAdapterMessage function.
func (ma *MockAdapterProxy) SendInterAdapterMessage(ctx context.Context,
	msg proto.Message,
	msgType inter_container.InterAdapterMessageType_Types,
	fromAdapter string,
	toAdapter string,
	toDeviceID string,
	proxyDeviceID string,
	messageID string) error {
	//panic("implement me")
	if ctx == nil || msg == nil || fromAdapter != "" ||
		toAdapter != "" || toDeviceID != "" || proxyDeviceID != "" || messageID != "" {
		return errors.New("sendInterAdapterMessage func parameters cannot be nil")
	}
	return nil
}
