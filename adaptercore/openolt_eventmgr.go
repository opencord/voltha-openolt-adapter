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
	"strconv"

	"github.com/opencord/voltha-lib-go/v2/pkg/adapters/adapterif"
	"github.com/opencord/voltha-lib-go/v2/pkg/log"
	oop "github.com/opencord/voltha-protos/v2/go/openolt"
	"github.com/opencord/voltha-protos/v2/go/voltha"
)

const (
	onuDiscoveryEvent       = "ONU_DISCOVERY"
	onuLosEvent             = "ONU_LOSS_OF_SIGNAL"
	onuLobEvent             = "ONU_LOSS_OF_BURST"
	onuLopcMissEvent        = "ONU_LOPC_MISS"
	onuLopcMicErrorEvent    = "ONU_LOPC_MIC_ERROR"
	oltLosEvent             = "OLT_LOSS_OF_SIGNAL"
	oltIndicationDown       = "OLT_DOWN_INDICATION"
	onuDyingGaspEvent       = "ONU_DYING_GASP"
	onuSignalsFailEvent     = "ONU_SIGNALS_FAIL"
	onuStartupFailEvent     = "ONU_STARTUP_FAIL"
	onuSignalDegradeEvent   = "ONU_SIGNAL_DEGRADE"
	onuDriftOfWindowEvent   = "ONU_DRIFT_OF_WINDOW"
	onuActivationFailEvent  = "ONU_ACTIVATION_FAIL"
	onuProcessingErrorEvent = "ONU_PROCESSING_ERROR"
	onuTiwiEvent            = "ONU_TRANSMISSION_WARNING"
	onuLossOmciEvent        = "ONU_LOSS_OF_OMCI_CHANNEL"
	onuLossOfKeySyncEvent   = "ONU_LOSS_OF_KEY_SYNC"
	onuLossOfFrameEvent     = "ONU_LOSS_OF_FRAME"
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
	ON   = "on"    // string "on" is represented as const keyword ON which is used for status check
	OFF  = "off"   // string "off" is represented as const keyword OFF which is used for status check
	UP   = "up"    // string "up" is represented as const keyword UP which is used for operation state
	DOWN = "down"  // string "down" is represented as const keyword DOWN which is used for operation state
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
	case *oop.AlarmIndication_OnuLossOfSyncFailInd:
		log.Infow("Received onu Loss of Sync Fail indication ", log.Fields{"alarm_ind": alarmInd})
		em.onuLossOfSyncIndication(alarmInd.GetOnuLossOfSyncFailInd(), deviceID, raisedTs)
	case *oop.AlarmIndication_OnuItuPonStatsInd:
		log.Infow("Received onu Itu Pon Stats indication ", log.Fields{"alarm_ind": alarmInd})
		log.Infow("Not implemented yet", log.Fields{"alarm_ind": alarmInd})
	default:
		log.Errorw("Received unknown indication type", log.Fields{"alarm_ind": alarmInd})

	}
}

// oltUpDownIndication handles Up and Down state of an OLT
func (em *OpenOltEventMgr) oltUpDownIndication(oltIndication *oop.OltIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["oper-state"] = oltIndication.OperState
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if oltIndication.OperState == DOWN {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltIndicationDown, "RAISE_EVENT")
	} else if oltIndication.OperState == UP {
		de.DeviceEventName = fmt.Sprintf("%s_%s", oltIndicationDown, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, communication, olt, raisedTs); err != nil {
		log.Errorw("Failed to send OLT event", log.Fields{"err": err})
	}
	log.Infow("OLT UpDown event sent to KAFKA", log.Fields{})
}

// OnuDiscoveryIndication is an exported method to handle ONU discovery event
func (em *OpenOltEventMgr) OnuDiscoveryIndication(onuDisc *oop.OnuDiscIndication, deviceID string, OnuID uint32, serialNumber string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(OnuID), 10)
	context["intf-id"] = strconv.FormatUint(uint64(onuDisc.IntfId), 10)
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
	context["intf-id"] = strconv.FormatUint(uint64(oltLos.IntfId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if oltLos.Status == ON {
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
	var serialNumber string
	context := make(map[string]string)
	/* Populating event context */
	serialNumber = ""
	onu := em.handler.formOnuKey(dgi.IntfId, dgi.OnuId)
	if onu, ok := em.handler.onus.Load(onu); ok {
		serialNumber = onu.(*OnuDevice).serialNumber
	}
	context["serial-number"] = serialNumber
	context["intf-id"] = strconv.FormatUint(uint64(dgi.IntfId), 10)
	context["onu-id"] = strconv.FormatUint(uint64(dgi.OnuId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if dgi.Status == ON {
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
	context["intf-id"] = strconv.FormatUint(uint64(onuAlarm.IntfId), 10)
	context["onu-id"] = strconv.FormatUint(uint64(onuAlarm.OnuId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuAlarm.LosStatus == ON {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLosEvent, "RAISE_EVENT")
	} else if onuAlarm.LosStatus == OFF {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLosEvent, "CLEAR_EVENT")
	} else if onuAlarm.LobStatus == ON {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLobEvent, "RAISE_EVENT")
	} else if onuAlarm.LobStatus == OFF {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLobEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMissStatus == ON {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMissEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMissStatus == OFF {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMissEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == ON {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMicErrorEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == OFF {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLopcMicErrorEvent, "CLEAR_EVENT")
	} else if onuAlarm.LofiStatus == ON {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfFrameEvent, "RAISE_EVENT")
	} else if onuAlarm.LofiStatus == OFF {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfFrameEvent, "CLEAR_EVENT")
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
	context["intf-id"] = strconv.FormatUint(uint64(oaf.IntfId), 10)
	context["onu-id"] = strconv.FormatUint(uint64(oaf.OnuId), 10)
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
	context["intf-id"] = strconv.FormatUint(uint64(onuLossOmci.IntfId), 10)
	context["onu-id"] = strconv.FormatUint(uint64(onuLossOmci.OnuId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLossOmci.Status == ON {
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
	context["intf-id"] = strconv.FormatUint(uint64(onuDriftWindow.IntfId), 10)
	context["onu-id"] = strconv.FormatUint(uint64(onuDriftWindow.OnuId), 10)
	context["drift"] = strconv.FormatUint(uint64(onuDriftWindow.OnuId), 10)
	context["new-eqd"] = strconv.FormatUint(uint64(onuDriftWindow.OnuId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuDriftWindow.Status == ON {
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
	context["intf-id"] = strconv.FormatUint(uint64(onuSignalDegrade.IntfId), 10)
	context["onu-id"] = strconv.FormatUint(uint64(onuSignalDegrade.OnuId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalDegrade.Status == ON {
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
	context["onu-id"] = strconv.FormatUint(uint64(onuSignalsFail.OnuId), 10)
	context["intf-id"] = strconv.FormatUint(uint64(onuSignalsFail.IntfId), 10)
	context["inverse-bit-error-rate"] = strconv.FormatUint(uint64(onuSignalsFail.InverseBitErrorRate), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuSignalsFail.Status == ON {
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

func (em *OpenOltEventMgr) onuLossOfSyncIndication(onuLOKI *oop.OnuLossOfKeySyncIndication, deviceID string, raisedTs int64) {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = strconv.FormatUint(uint64(onuLOKI.OnuId), 10)
	context["intf-id"] = strconv.FormatUint(uint64(onuLOKI.IntfId), 10)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceID
	if onuLOKI.Status == ON {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfKeySyncEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", onuLossOfKeySyncEvent, "CLEAR_EVENT")
	}

	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, security, onu, raisedTs); err != nil {
		log.Errorw("Failed to send onu loss of key sync event", log.Fields{"onu-id": onuLOKI.OnuId, "intf-id": onuLOKI.IntfId})
	}
	log.Infow("Onu loss of key sync event sent to kafka", log.Fields{"onu-id": onuLOKI.OnuId, "intf-id": onuLOKI.IntfId})
}
