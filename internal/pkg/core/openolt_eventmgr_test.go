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

// Package core provides APIs for the openOLT adapter
package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/opencord/voltha-openolt-adapter/pkg/mocks"
	oop "github.com/opencord/voltha-protos/v5/go/openolt"
)

func mockEventMgr() *OpenOltEventMgr {
	ep := &mocks.MockEventProxy{}
	dh := newMockDeviceHandler()
	dh.onus = sync.Map{}
	dh.onus.Store(dh.formOnuKey(1, 1), &OnuDevice{deviceID: "TEST_ONU",
		deviceType:   "ONU",
		serialNumber: "TEST_ONU_123",
		onuID:        1, intfID: 1})
	dh.onus.Store("1.3", NewOnuDevice("onu3", "onu3", "onu3", 1, 3, "onu3", false, "mock-endpoint"))
	dh.onus.Store("1.4", NewOnuDevice("onu4", "onu4", "onu4", 1, 4, "onu4", false, "mock-endpoint"))
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
		/* Pon Interface ID from Openolt agent is sent as port no while raising LoSIndication hence following same in test to have similar behavior
		   0x2 << 28 ^ 536870913 = 1 --> pon Intf Id*/
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_LosInd{LosInd: &oop.LosIndication{IntfId: 536870913, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_LosInd{LosInd: &oop.LosIndication{IntfId: 536870913}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_LosInd{LosInd: &oop.LosIndication{IntfId: 536870913, Status: "on"}}}}},

		// OnuAlarmIndication alams
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LosStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LosStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		// Duplicate test to get onu los already cleared result
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LosStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LobStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LobStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMissStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMissStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMicErrorStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LopcMicErrorStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LofiStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LofiStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LoamiStatus: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuAlarmInd{OnuAlarmInd: &oop.OnuAlarmIndication{IntfId: 1, OnuId: 3, LoamiStatus: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
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

		// AlarmIndication_onuLossOfKeySyncInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuLossOfSyncFailInd{OnuLossOfSyncFailInd: &oop.OnuLossOfKeySyncFailureIndication{IntfId: 1, OnuId: 3, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuLossOfSyncFailInd{OnuLossOfSyncFailInd: &oop.OnuLossOfKeySyncFailureIndication{IntfId: 1, OnuId: 3, Status: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},

		// AlarmIndication_onuDeactivationFailureInd
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuDeactivationFailureInd{OnuDeactivationFailureInd: &oop.OnuDeactivationFailureIndication{IntfId: 1, OnuId: 3, Status: "on"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuDeactivationFailureInd{OnuDeactivationFailureInd: &oop.OnuDeactivationFailureIndication{IntfId: 1, OnuId: 3, Status: "off"}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		// AlarmIndication_onuItuPonStatsIndication
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuItuPonStatsInd{OnuItuPonStatsInd: &oop.OnuItuPonStatsIndication{IntfId: 1, OnuId: 3, Stats: &oop.OnuItuPonStatsIndication_RdiErrorInd{RdiErrorInd: &oop.RdiErrorIndication{RdiErrorCount: 4, Status: "on"}}}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
		{"ProcessEvents-", args{alarmInd: &oop.AlarmIndication{Data: &oop.AlarmIndication_OnuItuPonStatsInd{OnuItuPonStatsInd: &oop.OnuItuPonStatsIndication{IntfId: 1, OnuId: 3, Stats: &oop.OnuItuPonStatsIndication_RdiErrorInd{RdiErrorInd: &oop.RdiErrorIndication{RdiErrorCount: 0, Status: "off"}}}}}, deviceID: "olt", raisedTs: time.Now().Unix()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			em.ProcessEvents(context.Background(), tt.args.alarmInd, tt.args.deviceID, tt.args.raisedTs)
		})
	}
}

func TestOpenOltEventMgr_OnuDiscoveryIndication(t *testing.T) {
	em := mockEventMgr()
	type args struct {
		onuDisc      *oop.OnuDiscIndication
		oltDeviceID  string
		onuDeviceID  string
		OnuID        uint32
		serialNumber string
		raisedTs     int64
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"OnuDiscoveryIndication", args{onuDisc: &oop.OnuDiscIndication{IntfId: 1, SerialNumber: &oop.SerialNumber{VendorId: []byte("TWSH"), VendorSpecific: []byte("1234")}}, oltDeviceID: "olt", onuDeviceID: "onu1", OnuID: 3, serialNumber: "1234", raisedTs: time.Now().Unix()}},
		{"OnuDiscoveryIndication", args{onuDisc: &oop.OnuDiscIndication{}, raisedTs: time.Now().Unix()}},
		{"OnuDiscoveryIndication", args{onuDisc: &oop.OnuDiscIndication{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = em.OnuDiscoveryIndication(context.Background(), tt.args.onuDisc, tt.args.oltDeviceID, tt.args.onuDeviceID, tt.args.OnuID, tt.args.serialNumber, tt.args.raisedTs)
			//TODO: actually verify test cases
		})
	}
}
