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

// Package adaptercore provides APIs for the openOLT adapter
package adaptercore

import (
	"testing"
	"time"

	"github.com/opencord/voltha-openolt-adapter/mocks"
	oop "github.com/opencord/voltha-protos/v2/go/openolt"
)

func mockEventMgr() *OpenOltEventMgr {
	ep := &mocks.MockEventProxy{}
	dh := &DeviceHandler{}
	dh.onus = make(map[string]*OnuDevice)
	dh.onus[dh.formOnuKey(1, 1)] = &OnuDevice{deviceID: "TEST_ONU",
		deviceType:   "ONU",
		serialNumber: "TEST_ONU_123",
		onuID:        1, intfID: 1}
	return NewEventMgr(ep, dh)
}
func TestOpenOltEventMgr_ProcessEvents(t *testing.T) {
	em := mockEventMgr()
	type args struct {
		alarmInd *oop.AlarmIndication
		deviceID string
		raisedTs int64
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		// LosIndication alarms
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_LosInd{LosInd: &oop.LosIndication{IntfId: 1, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_LosInd{LosInd: &oop.LosIndication{IntfId: 1}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_LosInd{LosInd: &oop.LosIndication{IntfId: 1, Status: "on"}}}}},

		// OnuAlarmIndication alams
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LosStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LosStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LobStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LobStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMissStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMissStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMicErrorStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMicErrorStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LosStatus: "on"}}}}},

		// AlarmIndication_DyingGaspInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_DyingGaspInd{DyingGaspInd: &oop.DyingGaspIndication{IntfId: 1, OnuId: 1, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_DyingGaspInd{DyingGaspInd: &oop.DyingGaspIndication{IntfId: 1, OnuId: 1, Status: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_DyingGaspInd{DyingGaspInd: &oop.DyingGaspIndication{IntfId: 1, OnuId: 1}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_DyingGaspInd{DyingGaspInd: &oop.DyingGaspIndication{IntfId: 1, OnuId: 1}}}}},

		// AlarmIndication_OnuActivationFailInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuActivationFailInd{OnuActivationFailInd: &oop.OnuActivationFailureIndication{IntfId: 1, OnuId: 3}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuActivationFailInd{OnuActivationFailInd: &oop.OnuActivationFailureIndication{IntfId: 1, OnuId: 3}}}}},

		// AlarmIndication_OnuLossOmciInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuLossOmciInd{OnuLossOmciInd: &oop.OnuLossOfOmciChannelIndication{IntfId: 1, OnuId: 3, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuLossOmciInd{OnuLossOmciInd: &oop.OnuLossOfOmciChannelIndication{IntfId: 1, OnuId: 3, Status: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuLossOmciInd{OnuLossOmciInd: &oop.OnuLossOfOmciChannelIndication{IntfId: 1, OnuId: 3, Status: "on"}}}}},

		// AlarmIndication_OnuDriftOfWindowInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuDriftOfWindowInd{OnuDriftOfWindowInd: &oop.OnuDriftOfWindowIndication{IntfId: 1, OnuId: 3, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuDriftOfWindowInd{OnuDriftOfWindowInd: &oop.OnuDriftOfWindowIndication{IntfId: 1, OnuId: 3, Status: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuDriftOfWindowInd{OnuDriftOfWindowInd: &oop.OnuDriftOfWindowIndication{IntfId: 1, OnuId: 3, Drift: 10, NewEqd: 10}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuDriftOfWindowInd{OnuDriftOfWindowInd: &oop.OnuDriftOfWindowIndication{IntfId: 1, OnuId: 3, Status: "on"}}}}},

		// AlarmIndication_OnuSignalDegradeInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalDegradeInd{OnuSignalDegradeInd: &oop.OnuSignalDegradeIndication{IntfId: 1, OnuId: 3, Status: "on", InverseBitErrorRate: 100}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalDegradeInd{OnuSignalDegradeInd: &oop.OnuSignalDegradeIndication{IntfId: 1, OnuId: 3, Status: "off", InverseBitErrorRate: 100}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalDegradeInd{OnuSignalDegradeInd: &oop.OnuSignalDegradeIndication{IntfId: 1, OnuId: 3, InverseBitErrorRate: 100}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalDegradeInd{OnuSignalDegradeInd: &oop.OnuSignalDegradeIndication{IntfId: 1, OnuId: 3, InverseBitErrorRate: 100}}}}},

		// AlarmIndication_OnuSignalsFailInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalsFailInd{OnuSignalsFailInd: &oop.OnuSignalsFailureIndication{IntfId: 1, OnuId: 3, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalsFailInd{OnuSignalsFailInd: &oop.OnuSignalsFailureIndication{IntfId: 1, OnuId: 3, Status: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalsFailInd{OnuSignalsFailInd: &oop.OnuSignalsFailureIndication{IntfId: 1, OnuId: 3, InverseBitErrorRate: 100}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuSignalsFailInd{OnuSignalsFailInd: &oop.OnuSignalsFailureIndication{IntfId: 1, OnuId: 3, InverseBitErrorRate: 100}}}}},

		// AlarmIndication_OnuProcessingErrorInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuProcessingErrorInd{OnuProcessingErrorInd: &oop.OnuProcessingErrorIndication{IntfId: 1, OnuId: 3}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuProcessingErrorInd{OnuProcessingErrorInd: &oop.OnuProcessingErrorIndication{IntfId: 1, OnuId: 3}}}}},

		// AlarmIndication_OnuTiwiInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuTiwiInd{OnuTiwiInd: &oop.OnuTransmissionInterferenceWarning{IntfId: 1, OnuId: 3, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuTiwiInd{OnuTiwiInd: &oop.OnuTransmissionInterferenceWarning{IntfId: 1, OnuId: 3, Status: "on", Drift: 100}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuTiwiInd{OnuTiwiInd: &oop.OnuTransmissionInterferenceWarning{IntfId: 1, OnuId: 3}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuTiwiInd{OnuTiwiInd: &oop.OnuTransmissionInterferenceWarning{IntfId: 1, OnuId: 3}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			em.ProcessEvents(tt.args.alarmInd, tt.args.deviceID, tt.args.raisedTs)
		})
	}
}

func TestOpenOltEventMgr_OnuDiscoveryIndication(t *testing.T) {
	em := mockEventMgr()
	type args struct {
		onuDisc      *oop.OnuDiscIndication
		deviceID     string
		OnuID        uint32
		serialNumber string
		raisedTs     int64
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"OnuDiscoveryIndication", args{onuDisc: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}, deviceID: "olt", OnuID: 3, serialNumber: "1234", raisedTs: time.Now().Unix()}},
		{"OnuDiscoveryIndication", args{onuDisc: &oop.OnuDiscIndication{}, raisedTs: time.Now().Unix()}},
		{"OnuDiscoveryIndication", args{onuDisc: &oop.OnuDiscIndication{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			em.OnuDiscoveryIndication(tt.args.onuDisc, tt.args.deviceID, tt.args.OnuID, tt.args.serialNumber, tt.args.raisedTs)
		})
	}
}
