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

// Package core provides the utility for olt devices, flows and statistics
package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	conf "github.com/opencord/voltha-lib-go/v7/pkg/config"
	"github.com/opencord/voltha-lib-go/v7/pkg/events/eventif"
	vgrpc "github.com/opencord/voltha-lib-go/v7/pkg/grpc"
	"github.com/opencord/voltha-lib-go/v7/pkg/log"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/config"
	"github.com/opencord/voltha-openolt-adapter/internal/pkg/olterrors"
	"github.com/opencord/voltha-protos/v5/go/adapter_service"
	"github.com/opencord/voltha-protos/v5/go/common"
	ca "github.com/opencord/voltha-protos/v5/go/core_adapter"
	"github.com/opencord/voltha-protos/v5/go/extension"
	"github.com/opencord/voltha-protos/v5/go/health"
	ia "github.com/opencord/voltha-protos/v5/go/inter_adapter"
	"github.com/opencord/voltha-protos/v5/go/omci"
	"github.com/opencord/voltha-protos/v5/go/voltha"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OpenOLT structure holds the OLT information
type OpenOLT struct {
	configManager                      *conf.ConfigManager
	deviceHandlers                     map[string]*DeviceHandler
	coreClient                         *vgrpc.Client
	eventProxy                         eventif.EventProxy
	config                             *config.AdapterFlags
	numOnus                            int
	KVStoreAddress                     string
	KVStoreType                        string
	exitChannel                        chan struct{}
	HeartbeatCheckInterval             time.Duration
	HeartbeatFailReportInterval        time.Duration
	GrpcTimeoutInterval                time.Duration
	lockDeviceHandlersMap              sync.RWMutex
	enableONUStats                     bool
	enableGemStats                     bool
	rpcTimeout                         time.Duration
	CheckOnuDevExistenceAtOnuDiscovery bool
}

// NewOpenOLT returns a new instance of OpenOLT
func NewOpenOLT(ctx context.Context,
	coreClient *vgrpc.Client,
	eventProxy eventif.EventProxy, cfg *config.AdapterFlags, cm *conf.ConfigManager) *OpenOLT {
	var openOLT OpenOLT
	openOLT.exitChannel = make(chan struct{})
	openOLT.deviceHandlers = make(map[string]*DeviceHandler)
	openOLT.config = cfg
	openOLT.numOnus = cfg.OnuNumber
	openOLT.coreClient = coreClient
	openOLT.eventProxy = eventProxy
	openOLT.KVStoreAddress = cfg.KVStoreAddress
	openOLT.KVStoreType = cfg.KVStoreType
	openOLT.HeartbeatCheckInterval = cfg.HeartbeatCheckInterval
	openOLT.HeartbeatFailReportInterval = cfg.HeartbeatFailReportInterval
	openOLT.GrpcTimeoutInterval = cfg.GrpcTimeoutInterval
	openOLT.lockDeviceHandlersMap = sync.RWMutex{}
	openOLT.configManager = cm
	openOLT.enableONUStats = cfg.EnableONUStats
	openOLT.enableGemStats = cfg.EnableGEMStats
	openOLT.rpcTimeout = cfg.RPCTimeout
	openOLT.CheckOnuDevExistenceAtOnuDiscovery = cfg.CheckOnuDevExistenceAtOnuDiscovery
	return &openOLT
}

// Start starts (logs) the device manager
func (oo *OpenOLT) Start(ctx context.Context) error {
	logger.Info(ctx, "starting-device-manager")
	logger.Info(ctx, "device-manager-started")
	return nil
}

// Stop terminates the session
func (oo *OpenOLT) Stop(ctx context.Context) error {
	logger.Info(ctx, "stopping-device-manager")
	close(oo.exitChannel)
	// Stop the device handlers
	oo.stopAllDeviceHandlers(ctx)

	// Stop the core grpc client connection
	if oo.coreClient != nil {
		oo.coreClient.Stop(ctx)
	}

	logger.Info(ctx, "device-manager-stopped")
	return nil
}

func (oo *OpenOLT) addDeviceHandlerToMap(agent *DeviceHandler) {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	if _, exist := oo.deviceHandlers[agent.device.Id]; !exist {
		oo.deviceHandlers[agent.device.Id] = agent
	}
}

func (oo *OpenOLT) deleteDeviceHandlerToMap(agent *DeviceHandler) {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	delete(oo.deviceHandlers, agent.device.Id)
}

func (oo *OpenOLT) getDeviceHandler(deviceID string) *DeviceHandler {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	if agent, ok := oo.deviceHandlers[deviceID]; ok {
		return agent
	}
	return nil
}

func (oo *OpenOLT) stopAllDeviceHandlers(ctx context.Context) {
	oo.lockDeviceHandlersMap.Lock()
	defer oo.lockDeviceHandlersMap.Unlock()
	for _, handler := range oo.deviceHandlers {
		handler.Stop(ctx)
	}
}

// AdoptDevice creates a new device handler if not present already and then adopts the device
func (oo *OpenOLT) AdoptDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	if device == nil {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil).Log()
	}
	logger.Infow(ctx, "adopt-device", log.Fields{"device-id": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		handler := NewDeviceHandler(oo.coreClient, oo.eventProxy, device, oo, oo.configManager, oo.config)
		oo.addDeviceHandlerToMap(handler)
		go handler.AdoptDevice(log.WithSpanFromContext(context.Background(), ctx), device)
	}
	return &empty.Empty{}, nil
}

// GetOfpDeviceInfo returns OFP information for the given device
func (oo *OpenOLT) GetOfpDeviceInfo(ctx context.Context, device *voltha.Device) (*ca.SwitchCapability, error) {
	logger.Infow(ctx, "get_ofp_device_info", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		return handler.GetOfpDeviceInfo(device)
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

// ReconcileDevice unimplemented
func (oo *OpenOLT) ReconcileDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	if device == nil {
		return nil, olterrors.NewErrInvalidValue(log.Fields{"device": nil}, nil)
	}
	logger.Infow(ctx, "reconcile-device", log.Fields{"device-id": device.Id})
	var handler *DeviceHandler
	if handler = oo.getDeviceHandler(device.Id); handler == nil {
		//Setting state to RECONCILING
		cgClient, err := oo.coreClient.GetCoreServiceClient()
		if err != nil {
			return nil, err
		}
		subCtx, cancel := context.WithTimeout(log.WithSpanFromContext(context.Background(), ctx), oo.rpcTimeout)
		defer cancel()
		if _, err := cgClient.DeviceStateUpdate(subCtx, &ca.DeviceStateFilter{
			DeviceId:   device.Id,
			OperStatus: voltha.OperStatus_RECONCILING,
			ConnStatus: device.ConnectStatus,
		}); err != nil {
			return nil, olterrors.NewErrAdapter("device-update-failed", log.Fields{"device-id": device.Id}, err)
		}

		// The OperState of the device is set to RECONCILING in the previous section. This also needs to be set on the
		// locally cached copy of the device struct.
		device.OperStatus = voltha.OperStatus_RECONCILING
		handler := NewDeviceHandler(oo.coreClient, oo.eventProxy, device, oo, oo.configManager, oo.config)
		handler.adapterPreviouslyConnected = true
		oo.addDeviceHandlerToMap(handler)
		handler.transitionMap = NewTransitionMap(handler)

		handler.transitionMap.Handle(log.WithSpanFromContext(context.Background(), ctx), DeviceInit)
	} else {
		logger.Warnf(ctx, "device-already-reconciled-or-active", log.Fields{"device-id": device.Id})
		return &empty.Empty{}, status.Errorf(codes.AlreadyExists, "handler exists: %s", device.Id)
	}
	return &empty.Empty{}, nil
}

// DisableDevice disables the given device
func (oo *OpenOLT) DisableDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	logger.Infow(ctx, "disable-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.DisableDevice(log.WithSpanFromContext(context.Background(), ctx), device); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

// ReEnableDevice enables the olt device after disable
func (oo *OpenOLT) ReEnableDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	logger.Infow(ctx, "reenable-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.ReenableDevice(log.WithSpanFromContext(context.Background(), ctx), device); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

// RebootDevice reboots the given device
func (oo *OpenOLT) RebootDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	logger.Infow(ctx, "reboot-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.RebootDevice(log.WithSpanFromContext(context.Background(), ctx), device); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)

}

// DeleteDevice deletes a device
func (oo *OpenOLT) DeleteDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	logger.Infow(ctx, "delete-device", log.Fields{"device-id": device.Id})
	if handler := oo.getDeviceHandler(device.Id); handler != nil {
		if err := handler.DeleteDevice(log.WithSpanFromContext(context.Background(), ctx), device); err != nil {
			return nil, err
		}
		// Initialize a ticker with a 2-second interval to periodically check the states
		// isHeartbeatCheckActive, isCollectorActive, and isReadIndicationRoutineActive.
		// Once all processes have stopped (all states are false), clear the device handler map
		// and exit the loop.
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			// Wait for the ticker to signal before checking the condition
			<-ticker.C
			if !handler.isHeartbeatCheckActive && !handler.isCollectorActive && !handler.isReadIndicationRoutineActive {
				logger.Debugf(ctx, "delete-device-handler")
				oo.deleteDeviceHandlerToMap(handler)
				break
			}
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": device.Id}, nil)
}

// UpdateFlowsIncrementally updates (add/remove) the flows on a given device
func (oo *OpenOLT) UpdateFlowsIncrementally(ctx context.Context, incrFlows *ca.IncrementalFlows) (*empty.Empty, error) {
	logger.Infow(ctx, "update_flows_incrementally", log.Fields{"device-id": incrFlows.Device.Id, "flows": incrFlows.Flows, "flowMetadata": incrFlows.FlowMetadata})
	if handler := oo.getDeviceHandler(incrFlows.Device.Id); handler != nil {
		if err := handler.UpdateFlowsIncrementally(log.WithSpanFromContext(context.Background(), ctx), incrFlows.Device, incrFlows.Flows, incrFlows.Groups, incrFlows.FlowMetadata); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": incrFlows.Device.Id}, nil)
}

// UpdatePmConfig returns PmConfigs nil or error
func (oo *OpenOLT) UpdatePmConfig(ctx context.Context, configs *ca.PmConfigsInfo) (*empty.Empty, error) {
	logger.Debugw(ctx, "update_pm_config", log.Fields{"device-id": configs.DeviceId, "pm-configs": configs.PmConfigs})
	if handler := oo.getDeviceHandler(configs.DeviceId); handler != nil {
		handler.UpdatePmConfig(log.WithSpanFromContext(context.Background(), ctx), configs.PmConfigs)
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": configs.DeviceId}, nil)
}

// SendPacketOut sends packet out to the device
func (oo *OpenOLT) SendPacketOut(ctx context.Context, packet *ca.PacketOut) (*empty.Empty, error) {
	logger.Debugw(ctx, "send_packet_out", log.Fields{"device-id": packet.DeviceId, "egress_port_no": packet.EgressPortNo, "pkt": packet.Packet})
	if handler := oo.getDeviceHandler(packet.DeviceId); handler != nil {
		if err := handler.PacketOut(log.WithSpanFromContext(context.Background(), ctx), packet.EgressPortNo, packet.Packet); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"device-id": packet.DeviceId}, nil)

}

// EnablePort to Enable PON/NNI interface
func (oo *OpenOLT) EnablePort(ctx context.Context, port *voltha.Port) (*empty.Empty, error) {
	logger.Infow(ctx, "enable_port", log.Fields{"device-id": port.DeviceId, "port": port})
	if err := oo.enableDisablePort(log.WithSpanFromContext(context.Background(), ctx), port.DeviceId, port, true); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// DisablePort to Disable pon/nni interface
func (oo *OpenOLT) DisablePort(ctx context.Context, port *voltha.Port) (*empty.Empty, error) {
	logger.Infow(ctx, "disable_port", log.Fields{"device-id": port.DeviceId, "port": port})
	if err := oo.enableDisablePort(log.WithSpanFromContext(context.Background(), ctx), port.DeviceId, port, false); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// enableDisablePort to Disable pon or Enable PON interface
func (oo *OpenOLT) enableDisablePort(ctx context.Context, deviceID string, port *voltha.Port, enablePort bool) error {
	logger.Infow(ctx, "enableDisablePort", log.Fields{"device-id": deviceID, "port": port})
	if port == nil {
		return olterrors.NewErrInvalidValue(log.Fields{
			"reason":    "port cannot be nil",
			"device-id": deviceID,
			"port":      nil}, nil)
	}
	if handler := oo.getDeviceHandler(deviceID); handler != nil {
		logger.Debugw(ctx, "Enable_Disable_Port", log.Fields{"device-id": deviceID, "port": port})
		if enablePort {
			if err := handler.EnablePort(ctx, port); err != nil {
				return olterrors.NewErrAdapter("error-occurred-during-enable-port", log.Fields{"device-id": deviceID, "port": port}, err)
			}
		} else {
			if err := handler.DisablePort(ctx, port); err != nil {
				return olterrors.NewErrAdapter("error-occurred-during-disable-port", log.Fields{"device-id": deviceID, "port": port}, err)
			}
		}
	}
	return nil
}

// ChildDeviceLost deletes the ONU and its references from PONResources
func (oo *OpenOLT) ChildDeviceLost(ctx context.Context, childDevice *voltha.Device) (*empty.Empty, error) {
	logger.Infow(ctx, "Child-device-lost", log.Fields{"parent-device-id": childDevice.ParentId, "child-device-id": childDevice.Id})
	if handler := oo.getDeviceHandler(childDevice.ParentId); handler != nil {
		if err := handler.ChildDeviceLost(log.WithSpanFromContext(context.Background(), ctx), childDevice.ParentPortNo, childDevice.ProxyAddress.OnuId, childDevice.SerialNumber); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("device-handler", log.Fields{"parent-device-id": childDevice.ParentId}, nil).Log()
}

// GetExtValue retrieves a value on a particular ONU
func (oo *OpenOLT) GetExtValue(ctx context.Context, extInfo *ca.GetExtValueMessage) (*extension.ReturnValues, error) {
	var err error
	resp := new(extension.ReturnValues)
	logger.Infow(ctx, "get_ext_value", log.Fields{"parent-device-id": extInfo.ParentDevice.Id, "onu-id": extInfo.ChildDevice.Id})
	if handler := oo.getDeviceHandler(extInfo.ParentDevice.Id); handler != nil {
		if resp, err = handler.getExtValue(ctx, extInfo.ChildDevice, extInfo.ValueType); err != nil {
			logger.Errorw(ctx, "error-occurred-during-get-ext-value",
				log.Fields{"parent-device-id": extInfo.ParentDevice.Id, "onu-id": extInfo.ChildDevice.Id, "error": err})
			return nil, err
		}
	}
	return resp, nil
}

// GetSingleValue handles get uni status on ONU and ondemand metric on OLT
func (oo *OpenOLT) GetSingleValue(ctx context.Context, request *extension.SingleGetValueRequest) (*extension.SingleGetValueResponse, error) {
	logger.Infow(ctx, "single_get_value_request", log.Fields{"request": request})

	errResp := func(status extension.GetValueResponse_Status,
		reason extension.GetValueResponse_ErrorReason) *extension.SingleGetValueResponse {
		return &extension.SingleGetValueResponse{
			Response: &extension.GetValueResponse{
				Status:    status,
				ErrReason: reason,
			},
		}
	}
	if handler := oo.getDeviceHandler(request.TargetId); handler != nil {
		switch reqType := request.GetRequest().GetRequest().(type) {
		case *extension.GetValueRequest_OltPortInfo:
			return handler.getOltPortCounters(ctx, reqType.OltPortInfo), nil
		case *extension.GetValueRequest_OnuPonInfo:
			return handler.getOnuPonCounters(ctx, reqType.OnuPonInfo), nil
		case *extension.GetValueRequest_RxPower:
			return handler.getRxPower(ctx, reqType.RxPower), nil
		case *extension.GetValueRequest_OltRxPower:
			return handler.getPONRxPower(ctx, reqType.OltRxPower), nil
		default:
			return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_UNSUPPORTED), nil
		}
	}

	logger.Infow(ctx, "Single_get_value_request failed ", log.Fields{"request": request})
	return errResp(extension.GetValueResponse_ERROR, extension.GetValueResponse_INVALID_DEVICE_ID), nil
}

/*
 *  OLT Inter-adapter service
 */

// ProxyOmciRequests proxies an onu sw download OMCI request from the child adapter
func (oo *OpenOLT) ProxyOmciRequests(ctx context.Context, request *ia.OmciMessages) (*empty.Empty, error) {
	if handler := oo.getDeviceHandler(request.ParentDeviceId); handler != nil {
		if err := handler.ProxyOmciRequests(ctx, request); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("no-device-handler", log.Fields{"parent-device-id": request.ParentDeviceId, "child-device-id": request.ChildDeviceId}, nil).Log()
}

// ProxyOmciRequest proxies an OMCI request from the child adapter
func (oo *OpenOLT) ProxyOmciRequest(ctx context.Context, request *ia.OmciMessage) (*empty.Empty, error) {
	logger.Debugw(ctx, "proxy-omci-request", log.Fields{"request": request})

	if handler := oo.getDeviceHandler(request.ParentDeviceId); handler != nil {
		if err := handler.ProxyOmciMessage(ctx, request); err != nil {
			return nil, err
		}
		return &empty.Empty{}, nil
	}
	return nil, olterrors.NewErrNotFound("no-device-handler", log.Fields{"parent-device-id": request.ParentDeviceId, "child-device-id": request.ChildDeviceId}, nil).Log()
}

// GetTechProfileInstance returns an instance of a tech profile
func (oo *OpenOLT) GetTechProfileInstance(ctx context.Context, request *ia.TechProfileInstanceRequestMessage) (*ia.TechProfileDownloadMessage, error) {
	logger.Debugw(ctx, "getting-tech-profile-request", log.Fields{"request": request})

	targetDeviceID := request.ParentDeviceId
	if targetDeviceID == "" {
		return nil, olterrors.NewErrNotFound("parent-id-empty", log.Fields{"parent-device-id": request.ParentDeviceId, "child-device-id": request.DeviceId}, nil).Log()
	}
	if handler := oo.getDeviceHandler(targetDeviceID); handler != nil {
		return handler.GetTechProfileDownloadMessage(ctx, request)
	}
	return nil, olterrors.NewErrNotFound("no-device-handler", log.Fields{"parent-device-id": request.ParentDeviceId, "child-device-id": request.DeviceId}, nil).Log()

}

// GetHealthStatus is used by a OltAdapterService client to detect a connection
// lost with the gRPC server hosting the OltAdapterService service
func (oo *OpenOLT) GetHealthStatus(stream adapter_service.AdapterService_GetHealthStatusServer) error {
	ctx := context.Background()
	logger.Debugw(ctx, "receive-stream-connection", log.Fields{"stream": stream})

	if stream == nil {
		return fmt.Errorf("conn-is-nil %v", stream)
	}
	initialRequestTime := time.Now()
	var remoteClient *common.Connection
	var tempClient *common.Connection
	var err error
loop:
	for {
		tempClient, err = stream.Recv()
		if err != nil {
			logger.Warnw(ctx, "received-stream-error", log.Fields{"remote-client": remoteClient, "error": err})
			break loop
		}
		// Send a response back
		err = stream.Send(&health.HealthStatus{State: health.HealthStatus_HEALTHY})
		if err != nil {
			logger.Warnw(ctx, "sending-stream-error", log.Fields{"remote-client": remoteClient, "error": err})
			break loop
		}

		remoteClient = tempClient
		logger.Debugw(ctx, "received-keep-alive", log.Fields{"remote-client": remoteClient})

		select {
		case <-stream.Context().Done():
			logger.Infow(ctx, "stream-keep-alive-context-done", log.Fields{"remote-client": remoteClient, "error": stream.Context().Err()})
			break loop
		case <-oo.exitChannel:
			logger.Warnw(ctx, "received-stop", log.Fields{"remote-client": remoteClient, "initial-conn-time": initialRequestTime})
			break loop
		default:
		}
	}
	logger.Errorw(ctx, "connection-down", log.Fields{"remote-client": remoteClient, "error": err, "initial-conn-time": initialRequestTime})
	return err
}

/*
 *
 * Unimplemented APIs
 *
 */

// SimulateAlarm is unimplemented
func (oo *OpenOLT) SimulateAlarm(context.Context, *ca.SimulateAlarmMessage) (*voltha.OperationResp, error) {
	return nil, olterrors.ErrNotImplemented
}

// SetExtValue is unimplemented
func (oo *OpenOLT) SetExtValue(context.Context, *ca.SetExtValueMessage) (*empty.Empty, error) {
	return nil, olterrors.ErrNotImplemented
}

// SetSingleValue is unimplemented
func (oo *OpenOLT) SetSingleValue(context.Context, *extension.SingleSetValueRequest) (*extension.SingleSetValueResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// StartOmciTest not implemented
func (oo *OpenOLT) StartOmciTest(ctx context.Context, test *ca.OMCITest) (*omci.TestResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// SuppressEvent unimplemented
func (oo *OpenOLT) SuppressEvent(ctx context.Context, filter *voltha.EventFilter) (*empty.Empty, error) {
	return nil, olterrors.ErrNotImplemented
}

// UnSuppressEvent  unimplemented
func (oo *OpenOLT) UnSuppressEvent(ctx context.Context, filter *voltha.EventFilter) (*empty.Empty, error) {
	return nil, olterrors.ErrNotImplemented
}

// DownloadImage is unimplemented
func (oo *OpenOLT) DownloadImage(ctx context.Context, imageInfo *ca.ImageDownloadMessage) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// GetImageDownloadStatus is unimplemented
func (oo *OpenOLT) GetImageDownloadStatus(ctx context.Context, imageInfo *ca.ImageDownloadMessage) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// CancelImageDownload is unimplemented
func (oo *OpenOLT) CancelImageDownload(ctx context.Context, imageInfo *ca.ImageDownloadMessage) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// ActivateImageUpdate is unimplemented
func (oo *OpenOLT) ActivateImageUpdate(ctx context.Context, imageInfo *ca.ImageDownloadMessage) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// RevertImageUpdate is unimplemented
func (oo *OpenOLT) RevertImageUpdate(ctx context.Context, imageInfo *ca.ImageDownloadMessage) (*voltha.ImageDownload, error) {
	return nil, olterrors.ErrNotImplemented
}

// DownloadOnuImage unimplemented
func (oo *OpenOLT) DownloadOnuImage(ctx context.Context, request *voltha.DeviceImageDownloadRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// GetOnuImageStatus unimplemented
func (oo *OpenOLT) GetOnuImageStatus(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// AbortOnuImageUpgrade unimplemented
func (oo *OpenOLT) AbortOnuImageUpgrade(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// GetOnuImages unimplemented
func (oo *OpenOLT) GetOnuImages(ctx context.Context, deviceID *common.ID) (*voltha.OnuImages, error) {
	return nil, olterrors.ErrNotImplemented
}

// ActivateOnuImage unimplemented
func (oo *OpenOLT) ActivateOnuImage(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// CommitOnuImage unimplemented
func (oo *OpenOLT) CommitOnuImage(ctx context.Context, in *voltha.DeviceImageRequest) (*voltha.DeviceImageResponse, error) {
	return nil, olterrors.ErrNotImplemented
}

// UpdateFlowsBulk is unimplemented
func (oo *OpenOLT) UpdateFlowsBulk(ctx context.Context, flows *ca.BulkFlows) (*empty.Empty, error) {
	return nil, olterrors.ErrNotImplemented
}

// SelfTestDevice unimplemented
func (oo *OpenOLT) SelfTestDevice(ctx context.Context, device *voltha.Device) (*empty.Empty, error) {
	return nil, olterrors.ErrNotImplemented
}
