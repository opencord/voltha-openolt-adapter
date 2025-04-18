/*
 * Copyright 2018-2024 Open Networking Foundation (ONF) and the ONF Contributors

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
	"errors"
	"fmt"
	"strconv"

	ca "github.com/opencord/voltha-protos/v5/go/core_adapter"

	"github.com/opencord/voltha-lib-go/v7/pkg/events/eventif"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	plt "github.com/opencord/voltha-lib-go/v7/pkg/platform"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"github.com/opencord/voltha-protos/v5/go/common"
	oop "github.com/opencord/voltha-protos/v5/go/openolt"
	"github.com/opencord/voltha-protos/v5/go/voltha"
)

const (
	onuDiscoveryEvent                   = "ONU_DISCOVERY"
	onuLosEvent                         = "ONU_LOSS_OF_SIGNAL"
	onuLobEvent                         = "ONU_LOSS_OF_BURST"
	onuLopcMissEvent                    = "ONU_LOPC_MISS"
	onuLopcMicErrorEvent                = "ONU_LOPC_MIC_ERROR"
	oltLosEvent                         = "OLT_LOSS_OF_SIGNAL"
	oltRebootFailedEvent                = "OLT_REBOOT_FAILED"
	oltCommFailure                      = "OLT_COMMUNICATION_FAILURE"
	oltIndicationDown                   = "OLT_DOWN_INDICATION"
	onuDyingGaspEvent                   = "ONU_DYING_GASP"
	onuSignalsFailEvent                 = "ONU_SIGNALS_FAIL"
	onuStartupFailEvent                 = "ONU_STARTUP_FAIL"
	onuSignalDegradeEvent               = "ONU_SIGNAL_DEGRADE"
	onuDriftOfWindowEvent               = "ONU_DRIFT_OF_WINDOW"
	onuActivationFailEvent              = "ONU_ACTIVATION_FAIL"
	onuLossOmciEvent                    = "ONU_LOSS_OF_OMCI_CHANNEL"
	onuLossOfKeySyncEvent               = "ONU_LOSS_OF_KEY_SYNC"
	onuLossOfFrameEvent                 = "ONU_LOSS_OF_FRAME"
	onuLossOfPloamEvent                 = "ONU_LOSS_OF_PLOAM"
	ponIntfDownIndiction                = "OLT_PON_INTERFACE_DOWN"
	onuDeactivationFailureEvent         = "ONU_DEACTIVATION_FAILURE"
	onuRemoteDefectIndication           = "ONU_REMOTE_DEFECT"
	onuLossOfGEMChannelDelineationEvent = "ONU_LOSS_OF_GEM_CHANNEL_DELINEATION"
	onuPhysicalEquipmentErrorEvent      = "ONU_PHYSICAL_EQUIPMENT_ERROR"
	onuLossOfAcknowledgementEvent       = "ONU_LOSS_OF_ACKNOWLEDGEMENT"
	onuDifferentialReachExceededEvent   = "ONU_DIFFERENTIAL_REACH_EXCEEDED"
)

const (
	// statusCheckOn represents status check On
	statusCheckOn = "on"
	// statusCheckOff represents status check Off
	statusCheckOff = "off"
	// operationStateUp represents operation state Up
	operationStateUp = "up"
	// operationStateDown represents operation state Down
	operationStateDown = "down"
	// base10 represents base 10 conversion
	base10 = 10
)

const (
	// ContextOltAdminState is for the admin state of the Olt in the context of the event
	ContextOltAdminState = "admin-state"
	// ContextOltConnectState is for the connect state of the Olt in the context of the event
	ContextOltConnectState = "connect-state"
	// ContextOltFailureReason is to report the reason of an operation failure in the Olt
	ContextOltFailureReason = "failure-reason"
	// ContextOltOperState is for the operational state of the Olt in the context of the event
	ContextOltOperState = "oper-state"
	// ContextOltVendor is for the Olt vendor in the context of the event
	ContextOltVendor = "vendor"
	// ContextOltType is for the Olt type in the context of the event
	ContextOltType = "type"
	// ContextOltParentID is for the Olt parent id in the context of the event
	ContextOltParentID = "parent-id"
	// ContextOltParentPortNo is for the Olt parent port no in the context of the event
	ContextOltParentPortNo = "parent-port-no"
	// ContextOltFirmwareVersion is for the Olt firmware version in the context of the event
	ContextOltFirmwareVersion = "firmware-version"
	// ContextOltHardwareVersion is for the Olt hardware version in the context of the event
	ContextOltHardwareVersion = "hardware-version"
	// ContextOltSerialNumber is for the serial number of the OLT
	ContextOltSerialNumber = "serial-number"
	// ContextOltMacAddress is for the OLT mac address
	ContextOltMacAddress = "mac-address"
	// ContextDeviceID is for the device id in the context of the event
	ContextDeviceID = "id"
	// ContextOnuOnuID is for the Onu Id in the context of the event
	ContextOnuOnuID = "onu-id"
	// ContextOnuPonIntfID is for the PON interface Id on which the Onu Event occurred
	ContextOnuPonIntfID = "intf-id"
	// ContextOnuSerialNumber is for the serial number of the ONU
	ContextOnuSerialNumber = "serial-number"
	// ContextOnuDeviceID is for the device id of the ONU generated by VOLTHA
	ContextOnuDeviceID = "onu-device-id"
	// ContextOltPonIntfID is for the PON interface Id on an OLT event
	ContextOltPonIntfID = "intf-id"
	// ContextOnuFailureReaseon is for the reason of failure of/at ONU indicated by the event
	ContextOnuFailureReaseon = "fail-reason"
	// ContextOnuDrift is for the drift of an ONU in the context of an event
	ContextOnuDrift = "drift"
	// ContextOnuNewEqd is for the New Eqd of an ONU in the context of an event
	ContextOnuNewEqd = "new-eqd"
	// ContextOnuInverseBitErrorRate is for the inverse bit error rate in the context of an ONU event
	ContextOnuInverseBitErrorRate = "inverse-bit-error-rate"
	// ContextOltPonIntfOperState is for the operational state of a PON port in the context of an OLT event
	ContextOltPonIntfOperState = "oper-state"
	// ContextOnuRemoteDefectIndicatorCount is for the rdi in the context of an ONU event
	ContextOnuRemoteDefectIndicatorCount = "rdi-count"
	// ContextOnuDelineationErrors is for the delineation errors if present in an ONU events context
	ContextOnuDelineationErrors = "delineation-errors"
	// ContextOnuDifferentialDistance is for the differential distance in an ONU event context
	ContextOnuDifferentialDistance = "differential-distance"
	// ContextOltPonTechnology is to indicate the pon-technology type, ie, 'GPON' or 'XGS-PON' (TODO check for combo?)
	ContextOltPonTechnology = "pon-technology"
	// ContextOltPortLabel is to indicate the string label of the pon-port, example: pon-0
	ContextOltPortLabel = "port-label"
)

// OpenOltEventMgr struct contains
type OpenOltEventMgr struct {
	eventProxy eventif.EventProxy
	handler    *DeviceHandler
}

// NewEventMgr is a Function to get a new event manager struct for the OpenOLT to process and publish OpenOLT event
func NewEventMgr(eventProxy eventif.EventProxy, handler *DeviceHandler) *OpenOltEventMgr {
	var em OpenOltEventMgr
	em.eventProxy = eventProxy
	em.handler = handler
	return &em
}

// ProcessEvents is function to process and publish OpenOLT event
// nolint: gocyclo
func (em *OpenOltEventMgr) ProcessEvents(ctx context.Context, alarmInd *oop.AlarmIndication, deviceID string, raisedTs int64) {
	var err error
	switch alarmInd.Data.(type) {
	case *oop.AlarmIndication_LosInd:
		logger.Debugw(ctx, "received-los-indication", log.Fields{"alarm-ind": alarmInd})
		err = em.oltLosIndication(ctx, alarmInd.GetLosInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuAlarmInd:
		logger.Debugw(ctx, "received-onu-alarm-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuAlarmIndication(ctx, alarmInd.GetOnuAlarmInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_DyingGaspInd:
		logger.Debugw(ctx, "received-dying-gasp-indication", log.Fields{"alarm-ind": alarmInd})
		err = em.onuDyingGaspIndication(ctx, alarmInd.GetDyingGaspInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuLossOmciInd:
		logger.Debugw(ctx, "received-onu-loss-omci-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuLossOmciIndication(ctx, alarmInd.GetOnuLossOmciInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuDriftOfWindowInd:
		logger.Debugw(ctx, "received-onu-drift-of-window-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuDriftOfWindowIndication(ctx, alarmInd.GetOnuDriftOfWindowInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuSignalDegradeInd:
		logger.Debugw(ctx, "received-onu-signal-degrade-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuSignalDegradeIndication(ctx, alarmInd.GetOnuSignalDegradeInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuSignalsFailInd:
		logger.Debugw(ctx, "received-onu-signal-fail-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuSignalsFailIndication(ctx, alarmInd.GetOnuSignalsFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuStartupFailInd:
		logger.Debugw(ctx, "received-onu-startup-fail-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuStartupFailedIndication(ctx, alarmInd.GetOnuStartupFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuTiwiInd:
		logger.Debugw(ctx, "received-onu-transmission-warning-indication ", log.Fields{"alarm-ind": alarmInd})
		logger.Warnw(ctx, "not-implemented-yet", log.Fields{"alarm-ind": "Onu-Transmission-indication"})
	case *oop.AlarmIndication_OnuLossOfSyncFailInd:
		logger.Debugw(ctx, "received-onu-loss-of-sync-fail-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuLossOfSyncIndication(ctx, alarmInd.GetOnuLossOfSyncFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuItuPonStatsInd:
		logger.Debugw(ctx, "received-onu-itu-pon-stats-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuItuPonStatsIndication(ctx, alarmInd.GetOnuItuPonStatsInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuDeactivationFailureInd:
		logger.Debugw(ctx, "received-onu-deactivation-failure-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuDeactivationFailureIndication(ctx, alarmInd.GetOnuDeactivationFailureInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuLossGemDelineationInd:
		logger.Debugw(ctx, "received-onu-loss-of-gem-channel-delineation-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuLossOfGEMChannelDelineationIndication(ctx, alarmInd.GetOnuLossGemDelineationInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuPhysicalEquipmentErrorInd:
		logger.Debugw(ctx, "received-onu-physical-equipment-error-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuPhysicalEquipmentErrorIndication(ctx, alarmInd.GetOnuPhysicalEquipmentErrorInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuLossOfAckInd:
		logger.Debugw(ctx, "received-onu-loss-of-acknowledgement-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuLossOfAcknowledgementIndication(ctx, alarmInd.GetOnuLossOfAckInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuDiffReachExceededInd:
		logger.Debugw(ctx, "received-onu-differential-reach-exceeded-indication ", log.Fields{"alarm-ind": alarmInd})
		err = em.onuDifferentialReachExceededIndication(ctx, alarmInd.GetOnuDiffReachExceededInd(), deviceID, raisedTs)
	default:
		err = olterrors.NewErrInvalidValue(log.Fields{"indication-type": alarmInd}, nil)
	}
	if err != nil {
		_ = olterrors.NewErrCommunication("publish-message", log.Fields{"indication-type": alarmInd}, err).LogAt(log.WarnLevel)
	}
}

func (em *OpenOltEventMgr) oltCommunicationEvent(ctx context.Context, device *voltha.Device, raisedTs int64) {
	if device == nil {
		logger.Warn(ctx, "device-is-nil-can't-send-olt-communication-failure-event")
		return
	}
	var de voltha.DeviceEvent
	context := make(map[string]string)
	context[ContextOltOperState] = device.OperStatus.String()
	context[ContextOltAdminState] = device.AdminState.String()
	context[ContextOltVendor] = device.Vendor
	context[ContextOltConnectState] = device.ConnectStatus.String()
	context[ContextOltType] = device.Type
	context[ContextOltParentID] = device.ParentId
	context[ContextOltParentPortNo] = fmt.Sprintf("%d", device.ParentPortNo)
	context[ContextDeviceID] = device.Id
	context[ContextOltFirmwareVersion] = device.FirmwareVersion
	context[ContextOltHardwareVersion] = device.HardwareVersion
	context[ContextOltSerialNumber] = device.SerialNumber
	context[ContextOltMacAddress] = device.MacAddress
	de.Context = context
	de.ResourceId = device.Id

	if device.ConnectStatus == voltha.ConnectStatus_UNREACHABLE {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltCommFailure, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltCommFailure, "CLEAR_EVENT")
	}

	if err := em.eventProxy.SendDeviceEvent(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_OLT, raisedTs); err != nil {
		logger.Errorw(ctx, "failed-to-send-olt-comm-failure-event", log.Fields{"err": err})
	}
	logger.Debugw(ctx, "olt-comm-failure-event-sent-to-kafka",
		log.Fields{
			"device-id":      device.Id,
			"connect-status": device.ConnectStatus,
		})
}

// oltUpDownIndication handles Up and Down state of an OLT
func (em *OpenOltEventMgr) oltUpDownIndication(ctx context.Context, oltIndication *oop.OltIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context[ContextOltOperState] = oltIndication.OperState
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if oltIndication.OperState == operationStateDown {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltIndicationDown, "RAISE_EVENT")
	} else if oltIndication.OperState == operationStateUp {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltIndicationDown, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_OLT, raisedTs); err != nil {
		return olterrors.NewErrCommunication("send-olt-event", log.Fields{"device-id": deviceID}, err)
	}
	logger.Debugw(ctx, "olt-updown-event-sent-to-kafka", log.Fields{})
	return nil
}

// OnuDiscoveryIndication is an exported method to handle ONU discovery event
func (em *OpenOltEventMgr) OnuDiscoveryIndication(ctx context.Context, onuDisc *oop.OnuDiscIndication, oltDeviceID string, onuDeviceID string, OnuID uint32, serialNumber string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(OnuID), base10)
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuDisc.IntfId), base10)
	context[ContextOnuSerialNumber] = serialNumber
	context[ContextOnuDeviceID] = onuDeviceID
	context[ContextOltPonTechnology] = em.handler.getPonTechnology(onuDisc.IntfId)
	context[ContextOltPortLabel], _ = GetportLabel(onuDisc.GetIntfId(), voltha.Port_PON_OLT)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = oltDeviceID
	de.DeviceEventName = fmt.Sprintf("%s_%s", onuDiscoveryEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_PON, raisedTs, onuDeviceID); err != nil {
		return olterrors.NewErrCommunication("send-onu-discovery-event",
			log.Fields{
				"serial-number": serialNumber,
				"intf-id":       onuDisc.IntfId}, err)
	}
	logger.Debugw(ctx, "onu-discovery-event-sent-to-kafka",
		log.Fields{
			"serial-number": serialNumber,
			"intf-id":       onuDisc.IntfId})
	return nil
}

func (em *OpenOltEventMgr) oltLosIndication(ctx context.Context, oltLos *oop.LosIndication, deviceID string, raisedTs int64) error {
	var err error = nil
	var de voltha.DeviceEvent
	var alarmInd oop.OnuAlarmIndication
	ponIntdID := plt.PortNoToIntfID(oltLos.IntfId, voltha.Port_PON_OLT)

	context := make(map[string]string)
	/* Populating event context */
	context[ContextOltPonIntfID] = strconv.FormatUint(uint64(ponIntdID), base10)
	context[ContextOltPortLabel], _ = GetportLabel(oltLos.IntfId, voltha.Port_PON_OLT)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if oltLos.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltLosEvent, "RAISE_EVENT")

		/* When PON cable disconnected from OLT, it was expected OnuAlarmIndication
		   with "los_status: on" should be raised for each Onu connected to the PON
		   but BAL does not raise this Alarm hence manually sending OnuLosRaise event
		   for all the ONU's connected to PON on receiving LoSIndication for PON */
		em.handler.onus.Range(func(Onukey interface{}, onuInCache interface{}) bool {
			if onuInCache.(*OnuDevice).intfID == ponIntdID {
				alarmInd.IntfId = ponIntdID
				alarmInd.OnuId = onuInCache.(*OnuDevice).onuID
				alarmInd.LosStatus = statusCheckOn
				err = em.onuAlarmIndication(ctx, &alarmInd, deviceID, raisedTs)
			}
			return true
		})
		if err != nil {
			/* Return if any error encountered while processing ONU LoS Event*/
			return err
		}
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltLosEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_PON, raisedTs); err != nil {
		return err
	}
	logger.Debugw(ctx, "olt-los-event-sent-to-kafka", log.Fields{"intf-id": ponIntdID})
	return nil
}

func (em *OpenOltEventMgr) oltRebootFailedEvent(ctx context.Context, deviceID string, reason string, raisedTs int64) error {
	de := voltha.DeviceEvent{
		Context:         map[string]string{ContextOltFailureReason: "olt-reboot-failed"},
		ResourceId:      deviceID,
		DeviceEventName: fmt.Sprintf("%s_%s", oltRebootFailedEvent, "RAISE_EVENT")}
	if err := em.eventProxy.SendDeviceEvent(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_OLT,
		raisedTs); err != nil {
		return olterrors.NewErrCommunication("send-olt-reboot-failed-event", log.Fields{
			"device-id": deviceID, "raised-ts": raisedTs, "reason": reason}, err)
	}
	logger.Debugw(ctx, "olt-reboot-failed-event-sent-to-kafka", log.Fields{
		"device-id": deviceID, "raised-ts": raisedTs, "reason": reason})
	return nil
}

func (em *OpenOltEventMgr) populateContextWithSerialDeviceID(context map[string]string, intfID, onuID uint32) string {
	var serialNumber = ""
	var onuDeviceID = ""
	onu := em.handler.formOnuKey(intfID, onuID)
	if onu, ok := em.handler.onus.Load(onu); ok {
		serialNumber = onu.(*OnuDevice).serialNumber
		onuDeviceID = onu.(*OnuDevice).deviceID
	}

	context[ContextOltPortLabel], _ = GetportLabel(intfID, voltha.Port_PON_OLT)
	context[ContextOnuSerialNumber] = serialNumber
	context[ContextOnuDeviceID] = onuDeviceID
	return onuDeviceID
}

func (em *OpenOltEventMgr) onuDyingGaspIndication(ctx context.Context, dgi *oop.DyingGaspIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	onuDeviceID := em.populateContextWithSerialDeviceID(context, dgi.IntfId, dgi.OnuId)

	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(dgi.IntfId), base10)
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(dgi.OnuId), base10)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID

	if dgi.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDyingGaspEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDyingGaspEvent, "CLEAR_EVENT")
	}

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-dying-gasp-event-sent-to-kafka", log.Fields{"intf-id": dgi.IntfId})
	return nil
}

// wasLosRaised checks whether los raised already. If already raised returns true else false
func (em *OpenOltEventMgr) wasLosRaised(ctx context.Context, onuAlarm *oop.OnuAlarmIndication) bool {
	onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
	if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
		logger.Debugw(ctx, "onu-device-found-in-cache.", log.Fields{"intfID": onuAlarm.IntfId, "onuID": onuAlarm.OnuId})

		if onuAlarm.LosStatus == statusCheckOn {
			if onuInCache.(*OnuDevice).losRaised {
				logger.Warnw(ctx, "onu-los-raised-already", log.Fields{"onu_id": onuAlarm.OnuId,
					"intf_id": onuAlarm.IntfId, "LosStatus": onuAlarm.LosStatus})
				return true
			}
			return false
		}
	}
	return true
}

// wasLosCleared checks whether los cleared already. If already cleared returns true else false
func (em *OpenOltEventMgr) wasLosCleared(ctx context.Context, onuAlarm *oop.OnuAlarmIndication) bool {
	onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
	if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
		logger.Debugw(ctx, "onu-device-found-in-cache.", log.Fields{"intfID": onuAlarm.IntfId, "onuID": onuAlarm.OnuId})

		if onuAlarm.LosStatus == statusCheckOff {
			if !onuInCache.(*OnuDevice).losRaised {
				logger.Warnw(ctx, "onu-los-cleared-already", log.Fields{"onu_id": onuAlarm.OnuId,
					"intf_id": onuAlarm.IntfId, "LosStatus": onuAlarm.LosStatus})
				return true
			}
			return false
		}
	}
	return true
}

func (em *OpenOltEventMgr) getDeviceEventName(onuAlarm *oop.OnuAlarmIndication) string {
	var deviceEventName string
	if onuAlarm.LosStatus == statusCheckOn {
		deviceEventName = fmt.Sprintf("%s_%s", onuLosEvent, "RAISE_EVENT")
	} else if onuAlarm.LosStatus == statusCheckOff {
		deviceEventName = fmt.Sprintf("%s_%s", onuLosEvent, "CLEAR_EVENT")
	} else if onuAlarm.LobStatus == statusCheckOn {
		deviceEventName = fmt.Sprintf("%s_%s", onuLobEvent, "RAISE_EVENT")
	} else if onuAlarm.LobStatus == statusCheckOff {
		deviceEventName = fmt.Sprintf("%s_%s", onuLobEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMissStatus == statusCheckOn {
		deviceEventName = fmt.Sprintf("%s_%s", onuLopcMissEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMissStatus == statusCheckOff {
		deviceEventName = fmt.Sprintf("%s_%s", onuLopcMissEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == statusCheckOn {
		deviceEventName = fmt.Sprintf("%s_%s", onuLopcMicErrorEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == statusCheckOff {
		deviceEventName = fmt.Sprintf("%s_%s", onuLopcMicErrorEvent, "CLEAR_EVENT")
	} else if onuAlarm.LofiStatus == statusCheckOn {
		deviceEventName = fmt.Sprintf("%s_%s", onuLossOfFrameEvent, "RAISE_EVENT")
	} else if onuAlarm.LofiStatus == statusCheckOff {
		deviceEventName = fmt.Sprintf("%s_%s", onuLossOfFrameEvent, "CLEAR_EVENT")
	} else if onuAlarm.LoamiStatus == statusCheckOn {
		deviceEventName = fmt.Sprintf("%s_%s", onuLossOfPloamEvent, "RAISE_EVENT")
	} else if onuAlarm.LoamiStatus == statusCheckOff {
		deviceEventName = fmt.Sprintf("%s_%s", onuLossOfPloamEvent, "CLEAR_EVENT")
	}
	return deviceEventName
}

func (em *OpenOltEventMgr) onuAlarmIndication(ctx context.Context, onuAlarm *oop.OnuAlarmIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent

	context := make(map[string]string)
	/* Populating event context */
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuAlarm.IntfId), base10)
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuAlarm.OnuId), base10)
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuAlarm.IntfId, onuAlarm.OnuId)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = em.getDeviceEventName(onuAlarm)

	switch onuAlarm.LosStatus {
	case statusCheckOn:
		if em.wasLosRaised(ctx, onuAlarm) {
			/* No need to raise Onu Los Event as it might have already raised
			   or Onu might have deleted */
			return nil
		}
		onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
		if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
			/* Update onu device with LoS raised state as true */
			em.handler.onus.Store(onuKey, NewOnuDevice(onuInCache.(*OnuDevice).deviceID, onuInCache.(*OnuDevice).deviceType,
				onuInCache.(*OnuDevice).serialNumber, onuInCache.(*OnuDevice).onuID, onuInCache.(*OnuDevice).intfID,
				onuInCache.(*OnuDevice).proxyDeviceID, true, onuInCache.(*OnuDevice).adapterEndpoint))
		}
	case statusCheckOff:
		if em.wasLosCleared(ctx, onuAlarm) {
			/* No need to clear Onu Los Event as it might have already cleared
			   or Onu might have deleted */
			return nil
		}
		onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
		if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
			/* Update onu device with LoS raised state as false */
			em.handler.onus.Store(onuKey, NewOnuDevice(onuInCache.(*OnuDevice).deviceID, onuInCache.(*OnuDevice).deviceType,
				onuInCache.(*OnuDevice).serialNumber, onuInCache.(*OnuDevice).onuID, onuInCache.(*OnuDevice).intfID,
				onuInCache.(*OnuDevice).proxyDeviceID, false, onuInCache.(*OnuDevice).adapterEndpoint))
		}
	}

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-los-event-sent-to-kafka", log.Fields{"onu-id": onuAlarm.OnuId, "intf-id": onuAlarm.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuActivationIndication(ctx context.Context, eventName string, onuInd *oop.OnuIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuInd.IntfId), base10)
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuInd.OnuId), base10)
	context[ContextOnuFailureReaseon] = onuInd.FailReason.String()

	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuInd.IntfId, onuInd.OnuId)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = eventName
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_PON, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-activation-failure-event-sent-to-kafka", log.Fields{"onu-id": onuInd.OnuId, "intf-id": onuInd.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOmciIndication(ctx context.Context, onuLossOmci *oop.OnuLossOfOmciChannelIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuLossOmci.IntfId), base10)
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuLossOmci.OnuId), base10)

	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuLossOmci.IntfId, onuLossOmci.OnuId)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLossOmci.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOmciEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOmciEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-loss-of-omci-channel-event-sent-to-kafka", log.Fields{"onu-id": onuLossOmci.OnuId, "intf-id": onuLossOmci.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuDriftOfWindowIndication(ctx context.Context, onuDriftWindow *oop.OnuDriftOfWindowIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuDriftWindow.IntfId), base10)
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuDriftWindow.OnuId), base10)
	context[ContextOnuDrift] = strconv.FormatUint(uint64(onuDriftWindow.Drift), base10)
	context[ContextOnuNewEqd] = strconv.FormatUint(uint64(onuDriftWindow.NewEqd), base10)

	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuDriftWindow.IntfId, onuDriftWindow.OnuId)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuDriftWindow.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDriftOfWindowEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDriftOfWindowEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-drift-of-window-event-sent-to-kafka", log.Fields{"onu-id": onuDriftWindow.OnuId, "intf-id": onuDriftWindow.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuSignalDegradeIndication(ctx context.Context, onuSignalDegrade *oop.OnuSignalDegradeIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuSignalDegrade.IntfId), base10)
	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuSignalDegrade.OnuId), base10)
	context[ContextOnuInverseBitErrorRate] = strconv.FormatUint(uint64(onuSignalDegrade.InverseBitErrorRate), base10)

	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuSignalDegrade.IntfId, onuSignalDegrade.OnuId)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalDegrade.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalDegradeEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalDegradeEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-signal-degrade-event-sent-to-kafka", log.Fields{"onu-id": onuSignalDegrade.OnuId, "intf-id": onuSignalDegrade.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuSignalsFailIndication(ctx context.Context, onuSignalsFail *oop.OnuSignalsFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuSignalsFail.IntfId, onuSignalsFail.OnuId)

	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuSignalsFail.OnuId), base10)
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuSignalsFail.IntfId), base10)
	context[ContextOnuInverseBitErrorRate] = strconv.FormatUint(uint64(onuSignalsFail.InverseBitErrorRate), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalsFail.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalsFailEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalsFailEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-signals-fail-event-sent-to-kafka", log.Fields{"onu-id": onuSignalsFail.OnuId, "intf-id": onuSignalsFail.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuStartupFailedIndication(ctx context.Context, onuStartupFail *oop.OnuStartupFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuStartupFail.IntfId, onuStartupFail.OnuId)

	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuStartupFail.OnuId), base10)
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuStartupFail.IntfId), base10)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuStartupFail.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuStartupFailEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuStartupFailEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_PON, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-startup-fail-event-sent-to-kafka", log.Fields{"onu-id": onuStartupFail.OnuId, "intf-id": onuStartupFail.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOfSyncIndication(ctx context.Context, onuLOKI *oop.OnuLossOfKeySyncFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuLOKI.IntfId, onuLOKI.OnuId)

	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuLOKI.OnuId), base10)
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuLOKI.IntfId), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLOKI.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfKeySyncEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfKeySyncEvent, "CLEAR_EVENT")
	}

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_SECURITY, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-loss-of-key-sync-event-sent-to-kafka", log.Fields{"onu-id": onuLOKI.OnuId, "intf-id": onuLOKI.IntfId})
	return nil
}

// oltIntfOperIndication handles Up and Down state of an OLT PON ports
func (em *OpenOltEventMgr) oltIntfOperIndication(ctx context.Context, ifindication *oop.IntfOperIndication, deviceID string, raisedTs int64) {
	portNo := plt.IntfIDToPortNo(ifindication.IntfId, voltha.Port_PON_OLT)
	if port, err := em.handler.getPortFromCore(ctx, &ca.PortFilter{
		DeviceId: deviceID,
		Port:     portNo,
	}); err != nil {
		logger.Warnw(ctx, "Error while fetching port object", log.Fields{"device-id": deviceID, "err": err})
	} else if port.AdminState != common.AdminState_ENABLED {
		logger.Debugw(ctx, "port-disable/enable-event-not-generated--the-port-is-not-enabled-by-operator", log.Fields{"device-id": deviceID, "port": port})
		return
	}
	/* Populating event context */
	context := map[string]string{ContextOltPonIntfOperState: ifindication.GetOperState()}
	/* Populating device event body */
	var de voltha.DeviceEvent
	de.Context = context
	de.ResourceId = deviceID

	if ifindication.GetOperState() == operationStateDown {
		de.DeviceEventName = fmt.Sprintf("%s_%s", ponIntfDownIndiction, "RAISE_EVENT")
	} else if ifindication.OperState == operationStateUp {
		de.DeviceEventName = fmt.Sprintf("%s_%s", ponIntfDownIndiction, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(ctx, &de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_OLT, raisedTs); err != nil {
		_ = olterrors.NewErrCommunication("send-olt-intf-oper-status-event", log.Fields{"device-id": deviceID, "intf-id": ifindication.IntfId, "oper-state": ifindication.OperState}, err).LogAt(log.WarnLevel)
		return
	}
	logger.Debug(ctx, "sent-olt-intf-oper-status-event-to-kafka")
}

func (em *OpenOltEventMgr) onuDeactivationFailureIndication(ctx context.Context, onuDFI *oop.OnuDeactivationFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuDFI.IntfId, onuDFI.OnuId)

	context[ContextOnuOnuID] = strconv.FormatUint(uint64(onuDFI.OnuId), base10)
	context[ContextOnuPonIntfID] = strconv.FormatUint(uint64(onuDFI.IntfId), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuDFI.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDeactivationFailureEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDeactivationFailureEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, &de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-deactivation-failure-event-sent-to-kafka", log.Fields{"onu-id": onuDFI.OnuId, "intf-id": onuDFI.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuRemoteDefectIndication(ctx context.Context, onuID uint32, intfID uint32, rdiCount uint64, status string, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		ContextOnuOnuID:                      strconv.FormatUint(uint64(onuID), base10),
		ContextOnuPonIntfID:                  strconv.FormatUint(uint64(intfID), base10),
		ContextOnuRemoteDefectIndicatorCount: strconv.FormatUint(rdiCount, base10),
	}
	onuDeviceID := em.populateContextWithSerialDeviceID(context, intfID, onuID)

	/* Populating device event body */
	de := &voltha.DeviceEvent{
		Context:    context,
		ResourceId: deviceID,
	}
	if status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuRemoteDefectIndication, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuRemoteDefectIndication, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-remote-defect-event-sent-to-kafka", log.Fields{"onu-id": onuID, "intf-id": intfID})
	return nil
}

func (em *OpenOltEventMgr) onuItuPonStatsIndication(ctx context.Context, onuIPS *oop.OnuItuPonStatsIndication, deviceID string, raisedTs int64) error {
	onuDevice, found := em.handler.onus.Load(em.handler.formOnuKey(onuIPS.IntfId, onuIPS.OnuId))
	if !found {
		return errors.New("unknown-onu-device")
	}
	if onuIPS.GetRdiErrorInd().Status == statusCheckOn {
		if !onuDevice.(*OnuDevice).rdiRaised {
			if err := em.onuRemoteDefectIndication(ctx, onuIPS.OnuId, onuIPS.IntfId, onuIPS.GetRdiErrorInd().RdiErrorCount, statusCheckOn, deviceID, raisedTs); err != nil {
				return err
			}
			onuDevice.(*OnuDevice).rdiRaised = true
			return nil
		}
		logger.Debugw(ctx, "onu-remote-defect-already-raised", log.Fields{"onu-id": onuIPS.OnuId, "intf-id": onuIPS.IntfId})
	} else {
		if err := em.onuRemoteDefectIndication(ctx, onuIPS.OnuId, onuIPS.IntfId, onuIPS.GetRdiErrorInd().RdiErrorCount, statusCheckOff, deviceID, raisedTs); err != nil {
			return err
		}
		onuDevice.(*OnuDevice).rdiRaised = false
	}
	return nil
}

func (em *OpenOltEventMgr) onuLossOfGEMChannelDelineationIndication(ctx context.Context, onuGCD *oop.OnuLossOfGEMChannelDelineationIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		ContextOnuOnuID:             strconv.FormatUint(uint64(onuGCD.OnuId), base10),
		ContextOnuPonIntfID:         strconv.FormatUint(uint64(onuGCD.IntfId), base10),
		ContextOnuDelineationErrors: strconv.FormatUint(uint64(onuGCD.DelineationErrors), base10),
	}
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuGCD.IntfId, onuGCD.OnuId)

	/* Populating device event body */
	de := &voltha.DeviceEvent{
		Context:    context,
		ResourceId: deviceID,
	}
	if onuGCD.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfGEMChannelDelineationEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfGEMChannelDelineationEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, de, voltha.EventCategory_COMMUNICATION, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-loss-of-gem-channel-delineation-event-sent-to-kafka", log.Fields{"onu-id": onuGCD.OnuId, "intf-id": onuGCD.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuPhysicalEquipmentErrorIndication(ctx context.Context, onuErr *oop.OnuPhysicalEquipmentErrorIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		ContextOnuOnuID:     strconv.FormatUint(uint64(onuErr.OnuId), base10),
		ContextOnuPonIntfID: strconv.FormatUint(uint64(onuErr.IntfId), base10),
	}
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuErr.IntfId, onuErr.OnuId)
	/* Populating device event body */
	de := &voltha.DeviceEvent{
		Context:    context,
		ResourceId: deviceID,
	}
	if onuErr.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuPhysicalEquipmentErrorEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuPhysicalEquipmentErrorEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-physical-equipment-error-event-sent-to-kafka", log.Fields{"onu-id": onuErr.OnuId, "intf-id": onuErr.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOfAcknowledgementIndication(ctx context.Context, onuLOA *oop.OnuLossOfAcknowledgementIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		ContextOnuOnuID:     strconv.FormatUint(uint64(onuLOA.OnuId), base10),
		ContextOnuPonIntfID: strconv.FormatUint(uint64(onuLOA.IntfId), base10),
	}
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuLOA.IntfId, onuLOA.OnuId)

	/* Populating device event body */
	de := &voltha.DeviceEvent{
		Context:    context,
		ResourceId: deviceID,
	}
	if onuLOA.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfAcknowledgementEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfAcknowledgementEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-physical-equipment-error-event-sent-to-kafka", log.Fields{"onu-id": onuLOA.OnuId, "intf-id": onuLOA.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuDifferentialReachExceededIndication(ctx context.Context, onuDRE *oop.OnuDifferentialReachExceededIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		ContextOnuOnuID:                strconv.FormatUint(uint64(onuDRE.OnuId), base10),
		ContextOnuPonIntfID:            strconv.FormatUint(uint64(onuDRE.IntfId), base10),
		ContextOnuDifferentialDistance: strconv.FormatUint(uint64(onuDRE.Distance), base10),
	}
	onuDeviceID := em.populateContextWithSerialDeviceID(context, onuDRE.IntfId, onuDRE.OnuId)

	/* Populating device event body */
	de := &voltha.DeviceEvent{
		Context:    context,
		ResourceId: deviceID,
	}
	if onuDRE.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDifferentialReachExceededEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDifferentialReachExceededEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEventWithKey(ctx, de, voltha.EventCategory_EQUIPMENT, voltha.EventSubCategory_ONU, raisedTs, onuDeviceID); err != nil {
		return err
	}
	logger.Debugw(ctx, "onu-differential-reach-exceeded–event-sent-to-kafka", log.Fields{"onu-id": onuDRE.OnuId, "intf-id": onuDRE.IntfId})
	return nil
}
