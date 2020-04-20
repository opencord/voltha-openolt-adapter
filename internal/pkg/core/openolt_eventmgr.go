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
	ctx "context"
	"fmt"
	"strconv"

	"github.com/opencord/voltha-lib-go/v3/pkg/adapters/adapterif"
	"github.com/opencord/voltha-lib-go/v3/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"github.com/opencord/voltha-protos/v3/go/common"
	oop "github.com/opencord/voltha-protos/v3/go/openolt"
	"github.com/opencord/voltha-protos/v3/go/voltha"
)

const (
	onuDiscoveryEvent                   = "ONU_DISCOVERY"
	onuLosEvent                         = "ONU_LOSS_OF_SIGNAL"
	onuLobEvent                         = "ONU_LOSS_OF_BURST"
	onuLopcMissEvent                    = "ONU_LOPC_MISS"
	onuLopcMicErrorEvent                = "ONU_LOPC_MIC_ERROR"
	oltLosEvent                         = "OLT_LOSS_OF_SIGNAL"
	oltIndicationDown                   = "OLT_DOWN_INDICATION"
	onuDyingGaspEvent                   = "ONU_DYING_GASP"
	onuSignalsFailEvent                 = "ONU_SIGNALS_FAIL"
	onuStartupFailEvent                 = "ONU_STARTUP_FAIL"
	onuSignalDegradeEvent               = "ONU_SIGNAL_DEGRADE"
	onuDriftOfWindowEvent               = "ONU_DRIFT_OF_WINDOW"
	onuActivationFailEvent              = "ONU_ACTIVATION_FAIL"
	onuProcessingErrorEvent             = "ONU_PROCESSING_ERROR"
	onuTiwiEvent                        = "ONU_TRANSMISSION_WARNING"
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
)

const (
	pon           = voltha.EventSubCategory_PON
	olt           = voltha.EventSubCategory_OLT
	ont           = voltha.EventSubCategory_ONT
	onu           = voltha.EventSubCategory_ONU
	nni           = voltha.EventSubCategory_NNI
	service       = voltha.EventCategory_SERVICE
	security      = voltha.EventCategory_SECURITY
	equipment     = voltha.EventCategory_EQUIPMENT
	processing    = voltha.EventCategory_PROCESSING
	environment   = voltha.EventCategory_ENVIRONMENT
	communication = voltha.EventCategory_COMMUNICATION
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
	// Threhold value for ONU RDI alarm
	onuITUPonAlarmThreshold = 3
)

// OpenOltEventMgr struct contains
type OpenOltEventMgr struct {
	eventProxy adapterif.EventProxy
	handler    *DeviceHandler
}

// NewEventMgr is a Function to get a new event manager struct for the OpenOLT to process and publish OpenOLT event
func NewEventMgr(eventProxy adapterif.EventProxy, handler *DeviceHandler) *OpenOltEventMgr {
	var em OpenOltEventMgr
	em.eventProxy = eventProxy
	em.handler = handler
	return &em
}

// ProcessEvents is function to process and publish OpenOLT event
// nolint: gocyclo
func (em *OpenOltEventMgr) ProcessEvents(alarmInd *oop.AlarmIndication, deviceID string, raisedTs int64) error {
	var err error
	switch alarmInd.Data.(type) {
	case *oop.AlarmIndication_LosInd:
		logger.Debugw("Received LOS indication", log.Fields{"alarm_ind": alarmInd})
		err = em.oltLosIndication(alarmInd.GetLosInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuAlarmInd:
		logger.Debugw("Received onu alarm indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuAlarmIndication(alarmInd.GetOnuAlarmInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_DyingGaspInd:
		logger.Debugw("Received dying gasp indication", log.Fields{"alarm_ind": alarmInd})
		err = em.onuDyingGaspIndication(alarmInd.GetDyingGaspInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuActivationFailInd:
		logger.Debugw("Received onu activation fail indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuActivationFailIndication(alarmInd.GetOnuActivationFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuLossOmciInd:
		logger.Debugw("Received onu loss omci indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuLossOmciIndication(alarmInd.GetOnuLossOmciInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuDriftOfWindowInd:
		logger.Debugw("Received onu drift of window indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuDriftOfWindowIndication(alarmInd.GetOnuDriftOfWindowInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuSignalDegradeInd:
		logger.Debugw("Received onu signal degrade indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuSignalDegradeIndication(alarmInd.GetOnuSignalDegradeInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuSignalsFailInd:
		logger.Debugw("Received onu signal fail indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuSignalsFailIndication(alarmInd.GetOnuSignalsFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuStartupFailInd:
		logger.Debugw("Received onu startup fail indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuStartupFailedIndication(alarmInd.GetOnuStartupFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuTiwiInd:
		logger.Debugw("Received onu transmission warning indication ", log.Fields{"alarm_ind": alarmInd})
		logger.Debugw("Not implemented yet", log.Fields{"alarm_ind": "Onu-Transmission-indication"})
	case *oop.AlarmIndication_OnuLossOfSyncFailInd:
		logger.Debugw("Received onu Loss of Sync Fail indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuLossOfSyncIndication(alarmInd.GetOnuLossOfSyncFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuItuPonStatsInd:
		logger.Debugw("Received onu Itu Pon Stats indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuItuPonStatsIndication(alarmInd.GetOnuItuPonStatsInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuDeactivationFailureInd:
		logger.Debugw("Received onu deactivation failure indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuDeactivationFailureIndication(alarmInd.GetOnuDeactivationFailureInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuLossGemDelineationInd:
		logger.Debugw("Received onu loss of GEM channel delineation indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuLossOfGEMChannelDelineationIndication(alarmInd.GetOnuLossGemDelineationInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuPhysicalEquipmentErrorInd:
		logger.Debugw("Received onu physical equipment error indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuPhysicalEquipmentErrorIndication(alarmInd.GetOnuPhysicalEquipmentErrorInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuLossOfAckInd:
		logger.Debugw("Received onu loss of acknowledgement indication ", log.Fields{"alarm_ind": alarmInd})
		err = em.onuLossOfAcknowledgementIndication(alarmInd.GetOnuLossOfAckInd(), deviceID, raisedTs)
	default:
		err = olterrors.NewErrInvalidValue(log.Fields{"indication-type": alarmInd}, nil)
	}
	if err != nil {
		return olterrors.NewErrCommunication("publish-message", log.Fields{"indication-type": alarmInd}, err).Log()
	}
	return nil
}

// oltUpDownIndication handles Up and Down state of an OLT
func (em *OpenOltEventMgr) oltUpDownIndication(oltIndication *oop.OltIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["oper-state"] = oltIndication.OperState
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if oltIndication.OperState == operationStateDown {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltIndicationDown, "RAISE_EVENT")
	} else if oltIndication.OperState == operationStateUp {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltIndicationDown, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, olt, raisedTs); err != nil {
		return olterrors.NewErrCommunication("send-olt-event", log.Fields{"device-id": deviceID}, err)
	}
	logger.Debugw("OLT UpDown event sent to KAFKA", log.Fields{})
	return nil
}

// OnuDiscoveryIndication is an exported method to handle ONU discovery event
func (em *OpenOltEventMgr) OnuDiscoveryIndication(onuDisc *oop.OnuDiscIndication, deviceID string, OnuID uint32, serialNumber string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(OnuID), base10)
	context["intf-id"] = strconv.FormatUint(uint64(onuDisc.IntfId), base10)
	context["serial-number"] = serialNumber
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = fmt.Sprintf("%s_%s", onuDiscoveryEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, equipment, pon, raisedTs); err != nil {
		return olterrors.NewErrCommunication("send-onu-discovery-event", log.Fields{"serial-number": serialNumber, "intf-id": onuDisc.IntfId}, err)
	}
	logger.Debugw("ONU discovery event sent to KAFKA", log.Fields{"serial-number": serialNumber, "intf-id": onuDisc.IntfId})
	return nil
}

func (em *OpenOltEventMgr) oltLosIndication(oltLos *oop.LosIndication, deviceID string, raisedTs int64) error {
	var err error = nil
	var de voltha.DeviceEvent
	var alarmInd oop.OnuAlarmIndication
	ponIntdID := PortNoToIntfID(oltLos.IntfId, voltha.Port_PON_OLT)

	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = strconv.FormatUint(uint64(oltLos.IntfId), base10)
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
				err = em.onuAlarmIndication(&alarmInd, deviceID, raisedTs)
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
	if err := em.eventProxy.SendDeviceEvent(&de, communication, olt, raisedTs); err != nil {
		return err
	}
	logger.Debugw("OLT LOS event sent to KAFKA", log.Fields{"intf-id": oltLos.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuDyingGaspIndication(dgi *oop.DyingGaspIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	var serialNumber string
	context := make(map[string]string)
	/* Populating event context */
	serialNumber = ""
	onu := em.handler.formOnuKey(dgi.IntfId, dgi.OnuId)
	if onu, ok := em.handler.onus.Load(onu); ok {
		serialNumber = onu.(*OnuDevice).serialNumber
	}
	context["serial-number"] = serialNumber
	context["intf-id"] = strconv.FormatUint(uint64(dgi.IntfId), base10)
	context["onu-id"] = strconv.FormatUint(uint64(dgi.OnuId), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = fmt.Sprintf("%s_%s", onuDyingGaspEvent, "EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU dying gasp event sent to KAFKA", log.Fields{"intf-id": dgi.IntfId})
	return nil
}

//wasLosRaised checks whether los raised already. If already raised returns true else false
func (em *OpenOltEventMgr) wasLosRaised(onuAlarm *oop.OnuAlarmIndication) bool {
	onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
	if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
		logger.Debugw("onu-device-found-in-cache.", log.Fields{"intfID": onuAlarm.IntfId, "onuID": onuAlarm.OnuId})

		if onuAlarm.LosStatus == statusCheckOn {
			if onuInCache.(*OnuDevice).losRaised {
				logger.Warnw("onu-los-raised-already", log.Fields{"onu_id": onuAlarm.OnuId,
					"intf_id": onuAlarm.IntfId, "LosStatus": onuAlarm.LosStatus})
				return true
			}
			return false
		}
	}
	return true
}

//wasLosCleared checks whether los cleared already. If already cleared returns true else false
func (em *OpenOltEventMgr) wasLosCleared(onuAlarm *oop.OnuAlarmIndication) bool {
	onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
	if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
		logger.Debugw("onu-device-found-in-cache.", log.Fields{"intfID": onuAlarm.IntfId, "onuID": onuAlarm.OnuId})

		if onuAlarm.LosStatus == statusCheckOff {
			if !onuInCache.(*OnuDevice).losRaised {
				logger.Warnw("onu-los-cleared-already", log.Fields{"onu_id": onuAlarm.OnuId,
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

func (em *OpenOltEventMgr) onuAlarmIndication(onuAlarm *oop.OnuAlarmIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	var serialNumber string
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = strconv.FormatUint(uint64(onuAlarm.IntfId), base10)
	context["onu-id"] = strconv.FormatUint(uint64(onuAlarm.OnuId), base10)
	serialNumber = ""
	onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
	if onu, ok := em.handler.onus.Load(onuKey); ok {
		serialNumber = onu.(*OnuDevice).serialNumber
	}
	context["serial-number"] = serialNumber

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = em.getDeviceEventName(onuAlarm)

	switch onuAlarm.LosStatus {
	case statusCheckOn:
		if em.wasLosRaised(onuAlarm) {
			/* No need to raise Onu Los Event as it might have already raised
			   or Onu might have deleted */
			return nil
		}
		onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
		if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
			/* Update onu device with LoS raised state as true */
			em.handler.onus.Store(onuKey, NewOnuDevice(onuInCache.(*OnuDevice).deviceID, onuInCache.(*OnuDevice).deviceType,
				onuInCache.(*OnuDevice).serialNumber, onuInCache.(*OnuDevice).onuID, onuInCache.(*OnuDevice).intfID,
				onuInCache.(*OnuDevice).proxyDeviceID, true))
		}
	case statusCheckOff:
		if em.wasLosCleared(onuAlarm) {
			/* No need to clear Onu Los Event as it might have already cleared
			   or Onu might have deleted */
			return nil
		}
		onuKey := em.handler.formOnuKey(onuAlarm.IntfId, onuAlarm.OnuId)
		if onuInCache, ok := em.handler.onus.Load(onuKey); ok {
			/* Update onu device with LoS raised state as false */
			em.handler.onus.Store(onuKey, NewOnuDevice(onuInCache.(*OnuDevice).deviceID, onuInCache.(*OnuDevice).deviceType,
				onuInCache.(*OnuDevice).serialNumber, onuInCache.(*OnuDevice).onuID, onuInCache.(*OnuDevice).intfID,
				onuInCache.(*OnuDevice).proxyDeviceID, false))
		}
	}

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU LOS event sent to KAFKA", log.Fields{"onu-id": onuAlarm.OnuId, "intf-id": onuAlarm.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuActivationFailIndication(oaf *oop.OnuActivationFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = strconv.FormatUint(uint64(oaf.IntfId), base10)
	context["onu-id"] = strconv.FormatUint(uint64(oaf.OnuId), base10)
	context["fail-reason"] = strconv.FormatUint(uint64(oaf.FailReason), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = fmt.Sprintf("%s_%s", onuActivationFailEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, equipment, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU activation failure event sent to KAFKA", log.Fields{"onu-id": oaf.OnuId, "intf-id": oaf.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOmciIndication(onuLossOmci *oop.OnuLossOfOmciChannelIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = strconv.FormatUint(uint64(onuLossOmci.IntfId), base10)
	context["onu-id"] = strconv.FormatUint(uint64(onuLossOmci.OnuId), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLossOmci.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOmciEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOmciEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU loss of OMCI channel event sent to KAFKA", log.Fields{"onu-id": onuLossOmci.OnuId, "intf-id": onuLossOmci.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuDriftOfWindowIndication(onuDriftWindow *oop.OnuDriftOfWindowIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = strconv.FormatUint(uint64(onuDriftWindow.IntfId), base10)
	context["onu-id"] = strconv.FormatUint(uint64(onuDriftWindow.OnuId), base10)
	context["drift"] = strconv.FormatUint(uint64(onuDriftWindow.Drift), base10)
	context["new-eqd"] = strconv.FormatUint(uint64(onuDriftWindow.NewEqd), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuDriftWindow.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDriftOfWindowEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDriftOfWindowEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU drift of window event sent to KAFKA", log.Fields{"onu-id": onuDriftWindow.OnuId, "intf-id": onuDriftWindow.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuSignalDegradeIndication(onuSignalDegrade *oop.OnuSignalDegradeIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = strconv.FormatUint(uint64(onuSignalDegrade.IntfId), base10)
	context["onu-id"] = strconv.FormatUint(uint64(onuSignalDegrade.OnuId), base10)
	context["inverse-bit-error-rate"] = strconv.FormatUint(uint64(onuSignalDegrade.InverseBitErrorRate), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalDegrade.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalDegradeEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalDegradeEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU signal degrade event sent to KAFKA", log.Fields{"onu-id": onuSignalDegrade.OnuId, "intf-id": onuSignalDegrade.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuSignalsFailIndication(onuSignalsFail *oop.OnuSignalsFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(onuSignalsFail.OnuId), base10)
	context["intf-id"] = strconv.FormatUint(uint64(onuSignalsFail.IntfId), base10)
	context["inverse-bit-error-rate"] = strconv.FormatUint(uint64(onuSignalsFail.InverseBitErrorRate), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalsFail.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalsFailEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalsFailEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU signals fail event sent to KAFKA", log.Fields{"onu-id": onuSignalsFail.OnuId, "intf-id": onuSignalsFail.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuStartupFailedIndication(onuStartupFail *oop.OnuStartupFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(onuStartupFail.OnuId), base10)
	context["intf-id"] = strconv.FormatUint(uint64(onuStartupFail.IntfId), base10)

	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuStartupFail.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuStartupFailEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuStartupFailEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU startup fail event sent to KAFKA", log.Fields{"onu-id": onuStartupFail.OnuId, "intf-id": onuStartupFail.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOfSyncIndication(onuLOKI *oop.OnuLossOfKeySyncFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(onuLOKI.OnuId), base10)
	context["intf-id"] = strconv.FormatUint(uint64(onuLOKI.IntfId), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLOKI.Status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfKeySyncEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfKeySyncEvent, "CLEAR_EVENT")
	}

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, security, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU loss of key sync event sent to KAFKA", log.Fields{"onu-id": onuLOKI.OnuId, "intf-id": onuLOKI.IntfId})
	return nil
}

// oltIntfOperIndication handles Up and Down state of an OLT PON ports
func (em *OpenOltEventMgr) oltIntfOperIndication(ifindication *oop.IntfOperIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	portID := IntfIDToPortNo(ifindication.IntfId, voltha.Port_PON_OLT)
	device, err := em.handler.coreProxy.GetDevice(ctx.Background(), deviceID, deviceID)
	if err != nil {
		return olterrors.NewErrAdapter("Error while fetching Device object", log.Fields{"DeviceId": deviceID}, err)
	}
	for _, port := range device.Ports {
		if port.PortNo == portID {
			// Events are suppressed if the Port Adminstate is not enabled.
			if port.AdminState != common.AdminState_ENABLED {
				logger.Debugw("Port disable/enable event not generated because, The port is not enabled by operator", log.Fields{"deviceId": deviceID, "port": port})
				return nil
			}
			break
		}
	}
	/* Populating event context */
	context["oper-state"] = ifindication.GetOperState()
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID

	if ifindication.GetOperState() == operationStateDown {
		de.DeviceEventName = fmt.Sprintf("%s_%s", ponIntfDownIndiction, "RAISE_EVENT")
	} else if ifindication.OperState == operationStateUp {
		de.DeviceEventName = fmt.Sprintf("%s_%s", ponIntfDownIndiction, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, olt, raisedTs); err != nil {
		return olterrors.NewErrCommunication("send-olt-intf-oper-status-event", log.Fields{"device-id": deviceID, "intf-id": ifindication.IntfId, "oper-state": ifindication.OperState}, err).Log()
	}
	logger.Debug("sent-olt-intf-oper-status-event-to-kafka")
	return nil
}

func (em *OpenOltEventMgr) onuDeactivationFailureIndication(onuDFI *oop.OnuDeactivationFailureIndication, deviceID string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(onuDFI.OnuId), base10)
	context["intf-id"] = strconv.FormatUint(uint64(onuDFI.IntfId), base10)
	context["failure-reason"] = strconv.FormatUint(uint64(onuDFI.FailReason), base10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = onuDeactivationFailureEvent

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, equipment, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU deactivation failure event sent to KAFKA", log.Fields{"onu-id": onuDFI.OnuId, "intf-id": onuDFI.IntfId})
	return nil
}
func (em *OpenOltEventMgr) onuRemoteDefectIndication(onuID uint32, intfID uint32, rdiErrors uint32, status string, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		"onu-id":  strconv.FormatUint(uint64(onuID), base10),
		"intf-id": strconv.FormatUint(uint64(intfID), base10),
		"rdi-errors": strconv.FormatUint(uint64(rdiErrors), base10),
	}
	/* Populating device event body */
	de := &voltha.DeviceEvent{
		Context:         context,
		ResourceId:      deviceID,
	}
	if status == statusCheckOn {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuRemoteDefectIndication, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuRemoteDefectIndication, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(de, equipment, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU remote defect event sent to KAFKA", log.Fields{"onu-id":onuID, "intf-id": intfID})
	return nil
}

func (em *OpenOltEventMgr) onuItuPonStatsIndication(onuIPS *oop.OnuItuPonStatsIndication, deviceID string, raisedTs int64) error {
	if onuIPS.RdiErrors > onuITUPonAlarmThreshold {
		if err := em.onuRemoteDefectIndication(onuIPS.OnuId, onuIPS.IntfId, onuIPS.RdiErrors, statusCheckOn, deviceID, raisedTs); err != nil {
			return err
		}
	} else {
		if err := em.onuRemoteDefectIndication(onuIPS.OnuId, onuIPS.IntfId, onuIPS.RdiErrors, statusCheckOff, deviceID, raisedTs); err != nil {
			return err
		}
	}
	return nil
}

func (em *OpenOltEventMgr) onuLossOfGEMChannelDelineationIndication(onuGCD *oop.OnuLossOfGEMChannelDelineationIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		"onu-id":             strconv.FormatUint(uint64(onuGCD.OnuId), base10),
		"intf-id":            strconv.FormatUint(uint64(onuGCD.IntfId), base10),
		"delineation-errors": strconv.FormatUint(uint64(onuGCD.DelineationErrors), base10),
	}
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
	if err := em.eventProxy.SendDeviceEvent(de, communication, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU loss of GEM channel delineation event sent to KAFKA", log.Fields{"onu-id": onuGCD.OnuId, "intf-id": onuGCD.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuPhysicalEquipmentErrorIndication(onuErr *oop.OnuPhysicalEquipmentErrorIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		"onu-id":  strconv.FormatUint(uint64(onuErr.OnuId), base10),
		"intf-id": strconv.FormatUint(uint64(onuErr.IntfId), base10),
	}
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
	if err := em.eventProxy.SendDeviceEvent(de, equipment, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU physical equipment error event sent to KAFKA", log.Fields{"onu-id": onuErr.OnuId, "intf-id": onuErr.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOfAcknowledgementIndication(onuLOA *oop.OnuLossOfAcknowledgementIndication, deviceID string, raisedTs int64) error {
	/* Populating event context */
	context := map[string]string{
		"onu-id":  strconv.FormatUint(uint64(onuLOA.OnuId), base10),
		"intf-id": strconv.FormatUint(uint64(onuLOA.IntfId), base10),
	}
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
	if err := em.eventProxy.SendDeviceEvent(de, equipment, onu, raisedTs); err != nil {
		return err
	}
	logger.Debugw("ONU physical equipment error event sent to KAFKA", log.Fields{"onu-id": onuLOA.OnuId, "intf-id": onuLOA.IntfId})
	return nil
}
