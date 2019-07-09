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
package adaptercore

import (
	"errors"
	"fmt"
	com "github.com/opencord/voltha-go/adapters/common"
	"github.com/opencord/voltha-go/common/log"
	oop "github.com/opencord/voltha-protos/go/openolt"
	"github.com/opencord/voltha-protos/go/voltha"
)

const (
	OnuDiscoveryEvent       = "ONU_DISCOVERY"
	OnuLosEvent             = "ONU_LOSS_OF_SIGNAL"
	OnuLobEvent             = "ONU_LOSS_OF_BURST"
	OnuLopcMissEvent        = "ONU_LOPC_MISS"
	OnuLopcMicErrorEvent    = "ONU_LOPC_MIC_ERROR"
	OltLosEvent             = "OLT_LOSS_OF_SIGNAL"
	OnuDyingGaspEvent       = "ONU_DYING_GASP"
	OnuSignalsFailEvent     = "ONU_SIGNALS_FAIL"
	OnuStartupFailEvent     = "ONU_STARTUP_FAIL"
	OnuSignalDegradeEvent   = "ONU_SIGNAL_DEGRADE"
	OnuDriftOfWindowEvent   = "ONU_DRIFT_OF_WINDOW"
	OnuActivationFailEvent  = "ONU_ACTIVATION_FAIL"
	OnuProcessingErrorEvent = "ONU_PROCESSING_ERROR"
	OnuTiwiEvent            = "ONU_TRANSMISSION_WARNING"
	OnuLossOmciEvent        = "ONU_LOSS_OF_OMCI_CHANNEL"
)

const (
	PON           = voltha.EventSubCategory_PON
	OLT           = voltha.EventSubCategory_OLT
	ONT           = voltha.EventSubCategory_ONT
	ONU           = voltha.EventSubCategory_ONU
	NNI           = voltha.EventSubCategory_NNI
	SERVICE       = voltha.EventCategory_SERVICE
	SECURITY      = voltha.EventCategory_SECURITY
	EQUIPMENT     = voltha.EventCategory_EQUIPMENT
	PROCESSING    = voltha.EventCategory_PROCESSING
	ENVIRONMENT   = voltha.EventCategory_ENVIRONMENT
	COMMUNICATION = voltha.EventCategory_COMMUNICATION
)

type OpenOltEventMgr struct {
	eventProxy *com.EventProxy
}

func NewEventMgr(eventProxy *com.EventProxy) *OpenOltEventMgr {
	var em OpenOltEventMgr
	em.eventProxy = eventProxy
	return &em
}

func (em *OpenOltEventMgr) ProcessEvents(alarmInd *oop.AlarmIndication, deviceId string, raisedTs int64) error {

	switch alarmInd.Data.(type) {
	case *oop.AlarmIndication_LosInd:
		log.Infow("Received LOS indication", log.Fields{"alarm_ind": alarmInd})
		if err := em.oltLosIndication(alarmInd.GetLosInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle olt los alarm", log.Fields{"alarm_ind": alarmInd})
			return err
		}
	case *oop.AlarmIndication_OnuAlarmInd:
		log.Infow("Received onu alarm indication ", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuAlarmIndication(alarmInd.GetOnuAlarmInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle onu alarm", log.Fields{"alarm_ind": alarmInd})
		}
	case *oop.AlarmIndication_DyingGaspInd:
		log.Infow("Received dying gasp indication", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuDyingGaspIndication(alarmInd.GetDyingGaspInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle dying gasp alarm", log.Fields{"alarm_ind": alarmInd})
		}
	case *oop.AlarmIndication_OnuActivationFailInd:
		log.Infow("Received onu activation fail indication ", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuActivationFailIndication(alarmInd.GetOnuActivationFailInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle onu activation failure alarm", log.Fields{"alarm_ind": alarmInd})
		}
	case *oop.AlarmIndication_OnuLossOmciInd:
		log.Infow("Received onu loss omci indication ", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuLossOmciIndication(alarmInd.GetOnuLossOmciInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle onu loss of omci channel alarm", log.Fields{"alarm_ind": alarmInd})
		}
	case *oop.AlarmIndication_OnuDriftOfWindowInd:
		log.Infow("Received onu drift of window indication ", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuDriftOfWindowIndication(alarmInd.GetOnuDriftOfWindowInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle onu drift of window alarm", log.Fields{"alarm_ind": alarmInd})
		}
	case *oop.AlarmIndication_OnuSignalDegradeInd:
		log.Infow("Received onu signal degrade indication ", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuSignalDegradeIndication(alarmInd.GetOnuSignalDegradeInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle onu signal degrade alarm", log.Fields{"alarm_ind": alarmInd})
		}
		log.Infow("Received onu signal degrade indication ", log.Fields{"alarm_ind": alarmInd})
	case *oop.AlarmIndication_OnuSignalsFailInd:
		log.Infow("Received onu signal fail indication ", log.Fields{"alarm_ind": alarmInd})
		if err := em.onuSignalsFailIndication(alarmInd.GetOnuSignalsFailInd(), deviceId, raisedTs); err != nil {
			log.Errorw("Failed to handle onu signal fail alarm", log.Fields{"alarm_ind": alarmInd})
		}
	case *oop.AlarmIndication_OnuProcessingErrorInd:
		log.Infow("Received onu startup fail indication ", log.Fields{"alarm_ind": alarmInd})
		log.Infow("Not implemented yet", log.Fields{"alarm_ind": alarmInd})
	case *oop.AlarmIndication_OnuTiwiInd:
		log.Infow("Received onu transmission warning indication ", log.Fields{"alarm_ind": alarmInd})
		log.Infow("Not implemented yet", log.Fields{"alarm_ind": "Onu-Transmission-indication"})
	default:
		log.Errorw("Received unknown indication type", log.Fields{"alarm_ind": alarmInd})
		return errors.New("unknown indication type")

	}
	return nil
}

func (em *OpenOltEventMgr) OnuDiscoveryIndication(onuDisc *oop.OnuDiscIndication, deviceId string, onuId uint32, serialNumber string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = string(onuId)
	context["intf-id"] = string(onuDisc.IntfId)
	context["serial-number"] = serialNumber
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	de.DeviceEventName = fmt.Sprintf("%s_%s", OnuDiscoveryEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, EQUIPMENT, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU discovery event", log.Fields{"serial-number": serialNumber, "intf-id": onuDisc.IntfId})
		return err
	}
	log.Infow("ONU discovery event sent to KAFKA", log.Fields{"serial-number": serialNumber, "intf-id": onuDisc.IntfId})
	return nil
}

func (em *OpenOltEventMgr) oltLosIndication(oltLos *oop.LosIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(oltLos.IntfId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if oltLos.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OltLosEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OltLosEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, OLT, raisedTs); err != nil {
		log.Errorw("Failed to send OLT loss of signal event", log.Fields{"intf-id": oltLos.IntfId})
		return err
	}
	log.Infow("OLT LOS event sent to KAFKA", log.Fields{"intf-id": oltLos.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuDyingGaspIndication(dgi *oop.DyingGaspIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(dgi.IntfId)
	context["onu-id"] = string(dgi.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if dgi.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuDyingGaspEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuDyingGaspEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU Dying gasp event", log.Fields{"intf-id": dgi.IntfId, "onu-id": dgi.OnuId})
		return err
	}
	log.Infow("ONU dying gasp event sent to KAFKA", log.Fields{"intf-id": dgi.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuAlarmIndication(onuAlarm *oop.OnuAlarmIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuAlarm.IntfId)
	context["onu-id"] = string(onuAlarm.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if onuAlarm.LosStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLosEvent, "RAISE_EVENT")
	} else if onuAlarm.LosStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLosEvent, "CLEAR_EVENT")
	} else if onuAlarm.LobStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLobEvent, "RAISE_EVENT")
	} else if onuAlarm.LobStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLobEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMissStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLopcMissEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMissStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLopcMissEvent, "CLEAR_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLopcMicErrorEvent, "RAISE_EVENT")
	} else if onuAlarm.LopcMicErrorStatus == "off" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLopcMicErrorEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, ONU, raisedTs); err != nil {
		log.Errorw("Failed to send ONU Los event", log.Fields{"onu-id": onuAlarm.OnuId, "intf-id": onuAlarm.IntfId})
		return err
	}
	log.Infow("ONU LOS event sent to KAFKA", log.Fields{"onu-id": onuAlarm.OnuId, "intf-id": onuAlarm.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuActivationFailIndication(oaf *oop.OnuActivationFailureIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(oaf.IntfId)
	context["onu-id"] = string(oaf.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	de.DeviceEventName = fmt.Sprintf("%s_%s", OnuActivationFailEvent, "RAISE_EVENT")
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, EQUIPMENT, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU activation failure event", log.Fields{"onu-id": oaf.OnuId, "intf-id": oaf.IntfId})
		return err
	}
	log.Infow("ONU activation failure event sent to KAFKA", log.Fields{"onu-id": oaf.OnuId, "intf-id": oaf.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuLossOmciIndication(onuLossOmci *oop.OnuLossOfOmciChannelIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuLossOmci.IntfId)
	context["onu-id"] = string(onuLossOmci.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if onuLossOmci.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLossOmciEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuLossOmciEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU loss of OMCI channel event", log.Fields{"onu-id": onuLossOmci.OnuId, "intf-id": onuLossOmci.IntfId})
		return err
	}
	log.Infow("ONU loss of OMCI channel event sent to KAFKA", log.Fields{"onu-id": onuLossOmci.OnuId, "intf-id": onuLossOmci.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuDriftOfWindowIndication(onuDriftWindow *oop.OnuDriftOfWindowIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuDriftWindow.IntfId)
	context["onu-id"] = string(onuDriftWindow.OnuId)
	context["drift"] = string(onuDriftWindow.OnuId)
	context["new-eqd"] = string(onuDriftWindow.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if onuDriftWindow.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuDriftOfWindowEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuDriftOfWindowEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU drift of window event", log.Fields{"onu-id": onuDriftWindow.OnuId, "intf-id": onuDriftWindow.IntfId})
		return err
	}
	log.Infow("ONU drift of window event sent to KAFKA", log.Fields{"onu-id": onuDriftWindow.OnuId, "intf-id": onuDriftWindow.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuSignalDegradeIndication(onuSignalDegrade *oop.OnuSignalDegradeIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["intf-id"] = string(onuSignalDegrade.IntfId)
	context["onu-id"] = string(onuSignalDegrade.OnuId)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if onuSignalDegrade.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuSignalDegradeEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuSignalDegradeEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU signals degrade event", log.Fields{"onu-id": onuSignalDegrade.OnuId, "intf-id": onuSignalDegrade.IntfId})
		return err
	}
	log.Infow("ONU signal degrade event sent to KAFKA", log.Fields{"onu-id": onuSignalDegrade.OnuId, "intf-id": onuSignalDegrade.IntfId})
	return nil
}

func (em *OpenOltEventMgr) onuSignalsFailIndication(onuSignalsFail *oop.OnuSignalsFailureIndication, deviceId string, raisedTs int64) error {
	var de voltha.DeviceEvent
	context := make(map[string]string)
	/* Populating event context */
	context["onu-id"] = string(onuSignalsFail.OnuId)
	context["intf-id"] = string(onuSignalsFail.IntfId)
	context["inverse-bit-error-rate"] = string(onuSignalsFail.InverseBitErrorRate)
	/* Populating device event body */
	de.Context = context
	de.ResourceId = deviceId
	if onuSignalsFail.Status == "on" {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuSignalsFailEvent, "RAISE_EVENT")
	} else {
		de.DeviceEventName = fmt.Sprintf("%s_%s", OnuSignalsFailEvent, "CLEAR_EVENT")
	}
	/* Send event to KAFKA */
	if err := em.eventProxy.SendDeviceEvent(&de, COMMUNICATION, PON, raisedTs); err != nil {
		log.Errorw("Failed to send ONU signals fail event", log.Fields{"onu-id": onuSignalsFail.OnuId, "intf-id": onuSignalsFail.IntfId})
		return err
	}
	log.Infow("ONU signals fail event sent to KAFKA", log.Fields{"onu-id": onuSignalsFail.OnuId, "intf-id": onuSignalsFail.IntfId})
	return nil
}
