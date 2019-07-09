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
	"fmt"
	com "github.com/opencord/voltha-go/adapters/common"
	"github.com/opencord/voltha-go/common/log"
	oop "github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
)

const (
	onuDiscoveryEvent       = "ONU_DISCOVERY"
	onuLosEvent             = "ONU_LOSS_OF_SIGNAL"
	onuLobEvent             = "ONU_LOSS_OF_BURST"
	onuLopcMissEvent        = "ONU_LOPC_MISS"
	onuLopcMicErrorEvent    = "ONU_LOPC_MIC_ERROR"
	oltLosEvent             = "OLT_LOSS_OF_SIGNAL"
	onuDyingGaspEvent       = "ONU_DYING_GASP"
	onuSignalsFailEvent     = "ONU_SIGNALS_FAIL"
	onuStartupFailEvent     = "ONU_STARTUP_FAIL"
	onuSignalDegradeEvent   = "ONU_SIGNAL_DEGRADE"
	onuDriftOfWindowEvent   = "ONU_DRIFT_OF_WINDOW"
	onuActivationFailEvent  = "ONU_ACTIVATION_FAIL"
	onuProcessingErrorEvent = "ONU_PROCESSING_ERROR"
	onuTiwiEvent            = "ONU_TRANSMISSION_WARNING"
	onuLossOmciEvent        = "ONU_LOSS_OF_OMCI_CHANNEL"
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

// OpenOltEventMgr struct contains
type OpenOltEventMgr struct {
	eventProxy *com.EventProxy
}

// NewEventMgr is a Function to get a new event manager struct for the OpenOLT to process and publish OpenOLT event
func NewEventMgr(eventProxy *com.EventProxy) *OpenOltEventMgr {
	var em OpenOltEventMgr
	em.eventProxy = eventProxy
	return &em
}

// ProcessEvents is function to process and publish OpenOLT event
func (em *OpenOltEventMgr) ProcessEvents(alarmInd *oop.AlarmIndication, deviceID string, raisedTs int64) {

	switch alarmInd.Data.(type) {
	case *oop.AlarmIndication_LosInd:
		log.Infow("Received LOS indication", log.Fields{"alarm_ind": alarmInd})
		em.oltLosIndication(alarmInd.GetLosInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuAlarmInd:
		log.Infow("Received onu alarm indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuAlarmIndication(alarmInd.GetOnuAlarmInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_DyingGaspInd:
		log.Infow("Received dying gasp indication", log.Fields{"alarm_ind": alarmInd})
		em.onuDyingGaspIndication(alarmInd.GetDyingGaspInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuActivationFailInd:
		log.Infow("Received onu activation fail indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuActivationFailIndication(alarmInd.GetOnuActivationFailInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuLossOmciInd:
		log.Infow("Received onu loss omci indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuLossOmciIndication(alarmInd.GetOnuLossOmciInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuDriftOfWindowInd:
		log.Infow("Received onu drift of window indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuDriftOfWindowIndication(alarmInd.GetOnuDriftOfWindowInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuSignalDegradeInd:
		log.Infow("Received onu signal degrade indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuSignalDegradeIndication(alarmInd.GetOnuSignalDegradeInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuSignalsFailInd:
		log.Infow("Received onu signal fail indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuSignalsFailIndication(alarmInd.GetOnuSignalsFailInd(), deviceID, raisedTs)

	case *oop.AlarmIndication_OnuProcessingErrorInd:
		log.Infow("Received onu startup fail indication ", log.Fields{"alarm_ind": alarmInd})
		log.Infow("Not implemented yet", log.Fields{"alarm_ind": alarmInd})
	case *oop.AlarmIndication_OnuTiwiInd:
		log.Infow("Received onu transmission warning indication ", log.Fields{"alarm_ind": alarmInd})
		log.Infow("Not implemented yet", log.Fields{"alarm_ind": "Onu-Transmission-indication"})
	default:
		log.Errorw("Received unknown indication type", log.Fields{"alarm_ind": alarmInd})

	}
}

// OnuDiscoveryIndication is an exported method to handle ONU discovery event
func (em *OpenOltEventMgr) OnuDiscoveryIndication(onuDisc *oop.OnuDiscIndication, deviceID string, OnuID uint32, serialNumber string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = string(OnuID)
	context["intf-id"] = string(onuDisc.IntfId)
	context["serial-number"] = serialNumber
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = fmt.Sprintf("%s_%s", onuDiscoveryEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, equipment, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU discovery event", log.Fields{"serial-number": serialNumber, "intf-id": onuDisc.IntfId})
	}
	log.Infow("ONU discovery event sent to KAFKA", log.Fields{"serial-number": serialNumber, "intf-id": onuDisc.IntfId})
}

func (em *OpenOltEventMgr) oltLosIndication(oltLos *oop.LosIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(oltLos.IntfId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if oltLos.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltLosEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltLosEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, olt, raisedTs); err != nil {
		log.Errorw("Failed to send OLT loss of signal event", log.Fields{"intf-id": oltLos.IntfId})
	}
	log.Infow("OLT LOS event sent to KAFKA", log.Fields{"intf-id": oltLos.IntfId})
}

func (em *OpenOltEventMgr) onuDyingGaspIndication(dgi *oop.DyingGaspIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(dgi.IntfId)
	context["onu-id"] = string(dgi.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if dgi.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDyingGaspEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDyingGaspEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU Dying gasp event", log.Fields{"intf-id": dgi.IntfId, "onu-id": dgi.OnuId})
	}
	log.Infow("ONU dying gasp event sent to KAFKA", log.Fields{"intf-id": dgi.IntfId})
}

func (em *OpenOltEventMgr) onuAlarmIndication(onuAlarm *oop.OnuAlarmIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuAlarm.IntfId)
	context["onu-id"] = string(onuAlarm.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuAlarm.LosStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLosEvent, "RAISE_EVENT")
	} else if onuAlarm.LosStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLosEvent, "CLEAR_EVENT")
	} else if onuAlarm.LobStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLobEvent, "RAISE_EVENT")
	} else if onuAlarm.LobStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLobEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMissStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMissEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMissStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMissEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMicErrorEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMicErrorEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, onu, raisedTs); err != nil {
		log.Errorw("Failed to send ONU Los event", log.Fields{"onu-id": onuAlarm.OnuId, "intf-id": onuAlarm.IntfId})
	}
	log.Infow("ONU LOS event sent to KAFKA", log.Fields{"onu-id": onuAlarm.OnuId, "intf-id": onuAlarm.IntfId})
}

func (em *OpenOltEventMgr) onuActivationFailIndication(oaf *oop.OnuActivationFailureIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(oaf.IntfId)
	context["onu-id"] = string(oaf.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	de.DeviceEventName = fmt.Sprintf("%s_%s", onuActivationFailEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, equipment, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU activation failure event", log.Fields{"onu-id": oaf.OnuId, "intf-id": oaf.IntfId})
	}
	log.Infow("ONU activation failure event sent to KAFKA", log.Fields{"onu-id": oaf.OnuId, "intf-id": oaf.IntfId})
}

func (em *OpenOltEventMgr) onuLossOmciIndication(onuLossOmci *oop.OnuLossOfOmciChannelIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuLossOmci.IntfId)
	context["onu-id"] = string(onuLossOmci.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLossOmci.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOmciEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOmciEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU loss of OMCI channel event", log.Fields{"onu-id": onuLossOmci.OnuId, "intf-id": onuLossOmci.IntfId})
	}
	log.Infow("ONU loss of OMCI channel event sent to KAFKA", log.Fields{"onu-id": onuLossOmci.OnuId, "intf-id": onuLossOmci.IntfId})
}

func (em *OpenOltEventMgr) onuDriftOfWindowIndication(onuDriftWindow *oop.OnuDriftOfWindowIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuDriftWindow.IntfId)
	context["onu-id"] = string(onuDriftWindow.OnuId)
	context["drift"] = string(onuDriftWindow.OnuId)
	context["new-eqd"] = string(onuDriftWindow.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuDriftWindow.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDriftOfWindowEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuDriftOfWindowEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU drift of window event", log.Fields{"onu-id": onuDriftWindow.OnuId, "intf-id": onuDriftWindow.IntfId})
	}
	log.Infow("ONU drift of window event sent to KAFKA", log.Fields{"onu-id": onuDriftWindow.OnuId, "intf-id": onuDriftWindow.IntfId})
}

func (em *OpenOltEventMgr) onuSignalDegradeIndication(onuSignalDegrade *oop.OnuSignalDegradeIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuSignalDegrade.IntfId)
	context["onu-id"] = string(onuSignalDegrade.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalDegrade.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalDegradeEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalDegradeEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU signals degrade event", log.Fields{"onu-id": onuSignalDegrade.OnuId, "intf-id": onuSignalDegrade.IntfId})
	}
	log.Infow("ONU signal degrade event sent to KAFKA", log.Fields{"onu-id": onuSignalDegrade.OnuId, "intf-id": onuSignalDegrade.IntfId})
}

func (em *OpenOltEventMgr) onuSignalsFailIndication(onuSignalsFail *oop.OnuSignalsFailureIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = string(onuSignalsFail.OnuId)
	context["intf-id"] = string(onuSignalsFail.IntfId)
	context["inverse-bit-error-rate"] = string(onuSignalsFail.InverseBitErrorRate)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalsFail.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalsFailEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuSignalsFailEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, pon, raisedTs); err != nil {
		log.Errorw("Failed to send ONU signals fail event", log.Fields{"onu-id": onuSignalsFail.OnuId, "intf-id": onuSignalsFail.IntfId})
	}
	log.Infow("ONU signals fail event sent to KAFKA", log.Fields{"onu-id": onuSignalsFail.OnuId, "intf-id": onuSignalsFail.IntfId})
}
