#
# Copyright 2018 the original author or authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
import threading
import binascii
import grpc
import socket
import re
import structlog
import time
from twisted.internet import reactor
from twisted.internet.defer import inlineCallbacks, returnValue
from scapy.layers.l2 import Ether, Dot1Q
from transitions import Machine

from voltha_protos import openolt_pb2_grpc, openolt_pb2

from pyvoltha.adapters.extensions.alarms.onu.onu_discovery_alarm import OnuDiscoveryAlarm

from pyvoltha.common.utils.nethelpers import mac_str_to_tuple
from voltha_protos.openflow_13_pb2 import OFPPS_LIVE, OFPPF_FIBER, \
    OFPPS_LINK_DOWN, OFPPF_1GB_FD, \
    OFPC_GROUP_STATS, OFPC_PORT_STATS, OFPC_TABLE_STATS, OFPC_FLOW_STATS, \
    ofp_switch_features, ofp_port, ofp_port_stats, ofp_desc
from pyvoltha.common.utils.registry import registry
from voltha_protos.common_pb2 import AdminState, OperStatus, ConnectStatus
from voltha_protos.device_pb2 import Port, Device
from voltha_protos.inter_container_pb2 import SwitchCapability, PortCapability, \
    InterAdapterMessageType, InterAdapterOmciMessage
from voltha_protos.logical_device_pb2 import LogicalDevice, LogicalPort


class OpenoltDevice(object):
    """
    OpenoltDevice state machine:

        null ----> init ------> connected -----> up -----> down
                   ^ ^             |             ^         | |
                   | |             |             |         | |
                   | +-------------+             +---------+ |
                   |                                         |
                   +-----------------------------------------+
    """
    # pylint: disable=too-many-instance-attributes
    # pylint: disable=R0904
    states = [
        'state_null',
        'state_init',
        'state_connected',
        'state_up',
        'state_down']

    transitions = [
        {'trigger': 'go_state_init',
         'source': ['state_null', 'state_connected', 'state_down'],
         'dest': 'state_init',
         'before': 'do_state_init',
         'after': 'post_init'},
        {'trigger': 'go_state_connected',
         'source': 'state_init',
         'dest': 'state_connected',
         'before': 'do_state_connected'},
        {'trigger': 'go_state_up',
         'source': ['state_connected', 'state_down'],
         'dest': 'state_up',
         'before': 'do_state_up'},
        {'trigger': 'go_state_down',
         'source': ['state_up'],
         'dest': 'state_down',
         'before': 'do_state_down',
         'after': 'post_down'}]

    def __init__(self, **kwargs):
        super(OpenoltDevice, self).__init__()

        self.core_proxy = kwargs['core_proxy']
        self.adapter_proxy = kwargs['adapter_proxy']
        self.device_num = kwargs['device_num']
        self.device = kwargs['device']

        self.platform_class = kwargs['support_classes']['platform']
        self.resource_mgr_class = kwargs['support_classes']['resource_mgr']
        self.flow_mgr_class = kwargs['support_classes']['flow_mgr']
        self.alarm_mgr_class = kwargs['support_classes']['alarm_mgr']
        self.stats_mgr_class = kwargs['support_classes']['stats_mgr']
        self.bw_mgr_class = kwargs['support_classes']['bw_mgr']

        self.seen_discovery_indications = []
        self.stub = None
        self.connected = False
        is_reconciliation = kwargs.get('reconciliation', False)
        self.device_id = self.device.id
        self.host_and_port = self.device.host_and_port
        self.extra_args = self.device.extra_args
        self.device_info = None
        self.log = structlog.get_logger(id=self.device_id,
                                        ip=self.host_and_port)

        self.log.info('openolt-device-init')

        # default device id and device serial number. If device_info provides better results, they will be updated
        self.dpid = kwargs.get('dp_id')
        self.serial_number = self.host_and_port  # FIXME

        # Device already set in the event of reconciliation
        if not is_reconciliation:
            self.log.info('updating-device')
            # It is a new device
            # Update device
            self.device.root = True
            self.device.connect_status = ConnectStatus.UNREACHABLE
            self.device.oper_status = OperStatus.ACTIVATING

        # If logical device does exist use it, else create one after connecting to device
        if self.device.parent_id:
            # logical device already exists
            self.logical_device_id = self.device.parent_id
            if is_reconciliation:
                self.adapter_agent.reconcile_logical_device(
                    self.logical_device_id)

        # Initialize the OLT state machine
        self.machine = Machine(model=self, states=OpenoltDevice.states,
                               transitions=OpenoltDevice.transitions,
                               send_event=True, initial='state_null')
        self.go_state_init()

    @inlineCallbacks
    def create_logical_device(self, device_info):
        dpid = device_info.device_id
        serial_number = device_info.device_serial_number

        if dpid is None: dpid = self.dpid
        if serial_number is None: serial_number = self.serial_number

        if dpid == None or dpid == '':
            uri = self.host_and_port.split(":")[0]
            try:
                socket.inet_pton(socket.AF_INET, uri)
                dpid = '00:00:' + self.ip_hex(uri)
            except socket.error:
                # this is not an IP
                dpid = self.stringToMacAddr(uri)

        if serial_number == None or serial_number == '':
            serial_number = self.host_and_port

        self.log.info('creating-openolt-logical-device', dp_id=dpid, serial_number=serial_number)

        mfr_desc = device_info.vendor
        sw_desc = device_info.firmware_version
        hw_desc = device_info.model
        if device_info.hardware_version: hw_desc += '-' + device_info.hardware_version

        # Create logical OF device
        ld = LogicalDevice(
            root_device_id=self.device_id,
            switch_features=ofp_switch_features(
                n_buffers=256,  # TODO fake for now
                n_tables=2,  # TODO ditto
                capabilities=(  # TODO and ditto
                        OFPC_FLOW_STATS
                        | OFPC_TABLE_STATS
                        | OFPC_PORT_STATS
                        | OFPC_GROUP_STATS
                )
            ),
            desc=ofp_desc(
                serial_num=serial_number
            )
        )
        ld_init = self.adapter_agent.create_logical_device(ld,
                                                           dpid=dpid)

        self.logical_device_id = ld_init.id

        ##Moved setting serial number outside of the logical_device function
        #device = yield self.adapter_agent.get_device(self.device_id)
        #device.serial_number = serial_number
        #yield self.adapter_agent.update_device(device)

        self.dpid = dpid
        self.serial_number = serial_number

        self.log.info('created-openolt-logical-device', logical_device_id=ld_init.id)

    def stringToMacAddr(self, uri):
        regex = re.compile('[^a-zA-Z]')
        uri = regex.sub('', uri)

        l = len(uri)
        if l > 6:
            uri = uri[0:6]
        else:
            uri = uri + uri[0:6 - l]

        return ":".join([hex(ord(x))[-2:] for x in uri])

    def do_state_init(self, event):
        # Initialize gRPC
        self.log.debug("grpc-host-port", self.host_and_port)
        self.channel = grpc.insecure_channel(self.host_and_port)
        self.channel_ready_future = grpc.channel_ready_future(self.channel)

        self.log.info('openolt-device-created', device_id=self.device_id)

    def post_init(self, event):
        self.log.debug('post_init')

        # We have reached init state, starting the indications thread

        # Catch RuntimeError exception
        try:
            # Start indications thread
            self.indications_thread_handle = threading.Thread(
                target=self.indications_thread)
            # Old getter/setter API for daemon; use it directly as a
            # property instead. The Jinkins error will happon on the reason of
            # Exception in thread Thread-1 (most likely raised # during
            # interpreter shutdown)
            self.log.debug('starting indications thread')
            self.indications_thread_handle.setDaemon(True)
            self.indications_thread_handle.start()
        except Exception as e:
            self.log.exception('post_init failed', e=e)

    @inlineCallbacks
    def do_state_connected(self, event):
        self.log.debug("do_state_connected")
        
        self.stub = openolt_pb2_grpc.OpenoltStub(self.channel)

        delay = 1
        while True:
            try:
                self.device_info = self.stub.GetDeviceInfo(openolt_pb2.Empty())
                break
            except Exception as e:
                reraise = True
                if delay > 120:
                    self.log.error("gRPC failure too many times")
                else:
                    self.log.warn("gRPC failure, retry in %ds: %s"
                                  % (delay, repr(e)))
                    time.sleep(delay)
                    delay += delay
                    reraise = False

                if reraise:
                    raise

        self.log.info('Device connected', device_info=self.device_info)

        # self.create_logical_device(device_info)
        self.logical_device_id = '0'

        serial_number = self.device_info.device_serial_number
        if serial_number is None: 
            serial_number = self.serial_number
        self.device.serial_number = serial_number
        
        self.serial_number = serial_number
        
        self.device.root = True
        self.device.vendor = self.device_info.vendor
        self.device.model = self.device_info.model
        self.device.hardware_version = self.device_info.hardware_version
        self.device.firmware_version = self.device_info.firmware_version

        # TODO: check for uptime and reboot if too long (VOL-1192)

        self.device.connect_status = ConnectStatus.REACHABLE
        self.device.mac_address = "AA:BB:CC:DD:EE:FF"
        yield self.core_proxy.device_update(self.device)

        self.resource_mgr = self.resource_mgr_class(self.device_id,
                                                    self.host_and_port,
                                                    self.extra_args,
                                                    self.device_info)
        self.platform = self.platform_class(self.log, self.resource_mgr)
        self.flow_mgr = self.flow_mgr_class(self.core_proxy, self.adapter_proxy, self.log,
                                            self.stub, self.device_id,
                                            self.logical_device_id,
                                            self.platform, self.resource_mgr)
        
        self.alarm_mgr = self.alarm_mgr_class(self.log, self.core_proxy,
                                              self.device_id,
                                              self.logical_device_id,
                                              self.platform,
                                              self.serial_number)
        self.stats_mgr = self.stats_mgr_class(self, self.log, self.platform)
        self.bw_mgr = self.bw_mgr_class(self.log, self.core_proxy)
        
        self.connected = True

    @inlineCallbacks
    def do_state_up(self, event):
        self.log.debug("do_state_up")

        yield self.core_proxy.device_state_update(self.device_id,
                                               connect_status=ConnectStatus.REACHABLE,
                                               oper_status=OperStatus.ACTIVE)
        self.log.debug("done_state_up")

    @inlineCallbacks
    def do_state_down(self, event):
        self.log.debug("do_state_down")
        yield self.core_proxy.device_state_update(self.device_id,
                                               connect_status=ConnectStatus.UNREACHABLE,
                                               oper_status=OperStatus.UNKNOWN)
        self.log.debug("done_state_down")

    # def post_up(self, event):
    #     self.log.debug('post-up')
    #     self.flow_mgr.reseed_flows()

    def post_down(self, event):
        self.log.debug('post_down')
        self.flow_mgr.reset_flows()

    def indications_thread(self):
        self.log.debug('starting-indications-thread')
        self.log.debug('connecting to olt', device_id=self.device_id)
        self.channel_ready_future.result()  # blocking call
        self.log.info('connected to olt', device_id=self.device_id)
        self.go_state_connected()

        # TODO: thread timing issue.  stub isnt ready yet from above go_state_connected (which doesnt block)
        # Don't continue until connected is done
        while (not self.connected):
            time.sleep(0.5)

        self.indications = self.stub.EnableIndication(openolt_pb2.Empty())

        while True:
            try:
                # get the next indication from olt
                ind = next(self.indications)
            except Exception as e:
                self.log.warn('gRPC connection lost', error=e)
                reactor.callFromThread(self.go_state_down)
                reactor.callFromThread(self.go_state_init)
                break
            else:
                self.log.debug("rx indication", indication=ind)

                # indication handlers run in the main event loop
                if ind.HasField('olt_ind'):
                    reactor.callFromThread(self.olt_indication, ind.olt_ind)
                elif ind.HasField('intf_ind'):
                    reactor.callFromThread(self.intf_indication, ind.intf_ind)
                elif ind.HasField('intf_oper_ind'):
                    reactor.callFromThread(self.intf_oper_indication,
                                           ind.intf_oper_ind)
                elif ind.HasField('onu_disc_ind'):
                    reactor.callFromThread(self.onu_discovery_indication,
                                           ind.onu_disc_ind)
                elif ind.HasField('onu_ind'):
                    reactor.callFromThread(self.onu_indication, ind.onu_ind)
                elif ind.HasField('omci_ind'):
                    reactor.callFromThread(self.omci_indication, ind.omci_ind)
                elif ind.HasField('pkt_ind'):
                    reactor.callFromThread(self.packet_indication, ind.pkt_ind)
                elif ind.HasField('port_stats'):
                    reactor.callFromThread(
                        self.stats_mgr.port_statistics_indication,
                        ind.port_stats)
                elif ind.HasField('flow_stats'):
                    reactor.callFromThread(
                        self.stats_mgr.flow_statistics_indication,
                        ind.flow_stats)
                elif ind.HasField('alarm_ind'):
                    reactor.callFromThread(self.alarm_mgr.process_alarms,
                                           ind.alarm_ind)
                else:
                    self.log.warn('unknown indication type')

    def olt_indication(self, olt_indication):
        if olt_indication.oper_state == "up":
            self.go_state_up()
        elif olt_indication.oper_state == "down":
            self.go_state_down()

    def intf_indication(self, intf_indication):
        self.log.debug("intf indication", intf_id=intf_indication.intf_id,
                       oper_state=intf_indication.oper_state)

        if intf_indication.oper_state == "up":
            oper_status = OperStatus.ACTIVE
        else:
            oper_status = OperStatus.DISCOVERED

        # add_port update the port if it exists
        self.add_port(intf_indication.intf_id, Port.PON_OLT, oper_status)

    def intf_oper_indication(self, intf_oper_indication):
        self.log.debug("Received interface oper state change indication",
                       intf_id=intf_oper_indication.intf_id,
                       type=intf_oper_indication.type,
                       oper_state=intf_oper_indication.oper_state)

        if intf_oper_indication.oper_state == "up":
            oper_state = OperStatus.ACTIVE
        else:
            oper_state = OperStatus.DISCOVERED

        if intf_oper_indication.type == "nni":

            # add_(logical_)port update the port if it exists
            self.add_port(intf_oper_indication.intf_id,
                                    Port.ETHERNET_NNI, oper_state)

        elif intf_oper_indication.type == "pon":
            # FIXME - handle PON oper state change
            pass

    @inlineCallbacks
    def onu_discovery_indication(self, onu_disc_indication):
        intf_id = onu_disc_indication.intf_id
        serial_number = onu_disc_indication.serial_number

        serial_number_str = self.stringify_serial_number(serial_number)

        self.log.debug("onu discovery indication", intf_id=intf_id,
                       serial_number=serial_number_str)

        if serial_number_str in self.seen_discovery_indications:
            self.log.debug("skipping-seen-onu-discovery-indication", intf_id=intf_id,
                           serial_number=serial_number_str)
            return
        else:
            self.seen_discovery_indications.append(serial_number_str)

        # Post ONU Discover alarm  20180809_0805
        try:
            OnuDiscoveryAlarm(self.alarm_mgr.alarms, pon_id=intf_id,
                              serial_number=serial_number_str).raise_alarm()
        except Exception as disc_alarm_error:
            self.log.exception("onu-discovery-alarm-error",
                               errmsg=disc_alarm_error.message)
            # continue for now.

        onu_device = yield self.core_proxy.get_child_device(
           self.device_id,
           serial_number=serial_number_str)

        if onu_device is None:
            try:
                onu_id = self.resource_mgr.get_onu_id(intf_id)
                if onu_id is None:
                    raise Exception("onu-id-unavailable")

                self.add_onu_device(
                    intf_id,
                    self.platform.intf_id_to_port_no(intf_id, Port.PON_OLT),
                    onu_id, serial_number)
                self.activate_onu(intf_id, onu_id, serial_number,
                                  serial_number_str)
            except Exception as e:
                self.log.exception('onu-activation-failed', e=e)

        else:
            if onu_device.connect_status != ConnectStatus.REACHABLE:
                yield self.core_proxy.device_state_update(onu_device.id, connect_status=ConnectStatus.REACHABLE)

            onu_id = onu_device.proxy_address.onu_id
            if onu_device.oper_status == OperStatus.DISCOVERED \
                    or onu_device.oper_status == OperStatus.ACTIVATING:
                self.log.debug("ignore onu discovery indication, \
                               the onu has been discovered and should be \
                               activating shorlty", intf_id=intf_id,
                               onu_id=onu_id, state=onu_device.oper_status)
            elif onu_device.oper_status == OperStatus.ACTIVE:
                self.log.warn("onu discovery indication whereas onu is \
                              supposed to be active",
                              intf_id=intf_id, onu_id=onu_id,
                              state=onu_device.oper_status)
            elif onu_device.oper_status == OperStatus.UNKNOWN:
                self.log.info("onu in unknown state, recovering from olt \
                              reboot probably, activate onu", intf_id=intf_id,
                              onu_id=onu_id, serial_number=serial_number_str)

                yield self.core_proxy.device_state_update(onu_device.id, oper_status=OperStatus.DISCOVERED)

                try:
                    self.activate_onu(intf_id, onu_id, serial_number,
                                      serial_number_str)
                except Exception as e:
                    self.log.error('onu-activation-error',
                                   serial_number=serial_number_str, error=e)
            else:
                self.log.warn('unexpected state', onu_id=onu_id,
                              onu_device_oper_state=onu_device.oper_status)

    @inlineCallbacks
    def onu_indication(self, onu_indication):
        self.log.debug("onu indication", intf_id=onu_indication.intf_id,
                       onu_id=onu_indication.onu_id,
                       serial_number=onu_indication.serial_number,
                       oper_state=onu_indication.oper_state,
                       admin_state=onu_indication.admin_state)
        try:
            serial_number_str = self.stringify_serial_number(
                onu_indication.serial_number)
        except Exception as e:
            serial_number_str = None

        if serial_number_str is not None:
            onu_device = yield self.core_proxy.get_child_device(
                self.device_id,
                serial_number=serial_number_str)
        else:
            onu_device = yield self.core_proxy.get_child_device(
                self.device_id,
                parent_port_no=self.platform.intf_id_to_port_no(
                    onu_indication.intf_id, Port.PON_OLT),
                onu_id=onu_indication.onu_id)

        if onu_device is None:
            self.log.error('onu not found', intf_id=onu_indication.intf_id,
                           onu_id=onu_indication.onu_id)
            return

        if self.platform.intf_id_from_pon_port_no(onu_device.parent_port_no) \
                != onu_indication.intf_id:
            self.log.warn('ONU-is-on-a-different-intf-id-now',
                          previous_intf_id=self.platform.intf_id_from_pon_port_no(
                              onu_device.parent_port_no),
                          current_intf_id=onu_indication.intf_id)
            # FIXME - handle intf_id mismatch (ONU move?)

        if onu_device.proxy_address.onu_id != onu_indication.onu_id:
            # FIXME - handle onu id mismatch
            self.log.warn('ONU-id-mismatch, can happen if both voltha and '
                          'the olt rebooted',
                          expected_onu_id=onu_device.proxy_address.onu_id,
                          received_onu_id=onu_indication.onu_id)

        # Admin state
        if onu_indication.admin_state == 'down':
            if onu_indication.oper_state != 'down':
                self.log.error('ONU-admin-state-down-and-oper-status-not-down',
                               oper_state=onu_indication.oper_state)
                # Forcing the oper state change code to execute
                onu_indication.oper_state = 'down'

            # Port and logical port update is taken care of by oper state block

        elif onu_indication.admin_state == 'up':
            pass

        else:
            self.log.warn('Invalid-or-not-implemented-admin-state',
                          received_admin_state=onu_indication.admin_state)

        self.log.debug('admin-state-dealt-with')

        # Operating state
        if onu_indication.oper_state == 'down':

            if onu_device.connect_status != ConnectStatus.UNREACHABLE:
                yield self.core_proxy.device_state_update(onu_device.id, connect_status=ConnectStatus.UNREACHABLE)

            # Move to discovered state
            self.log.debug('onu-oper-state-is-down')

            if onu_device.oper_status != OperStatus.DISCOVERED:
                yield self.core_proxy.device_state_update(onu_device.id, oper_status=OperStatus.DISCOVERED)

            self.log.debug('inter-adapter-send-onu-ind', onu_indication=onu_indication)

            # TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
            yield self.adapter_proxy.send_inter_adapter_message(
                msg=onu_indication,
                type=InterAdapterMessageType.ONU_IND_REQUEST,
                from_adapter="openolt",
                to_adapter=onu_device.type,
                to_device_id=onu_device.id
            )

        elif onu_indication.oper_state == 'up':

            if onu_device.connect_status != ConnectStatus.REACHABLE:
                yield self.core_proxy.device_state_update(onu_device.id, connect_status=ConnectStatus.REACHABLE)

            if onu_device.oper_status != OperStatus.DISCOVERED:
                self.log.debug("ignore onu indication",
                               intf_id=onu_indication.intf_id,
                               onu_id=onu_indication.onu_id,
                               state=onu_device.oper_status,
                               msg_oper_state=onu_indication.oper_state)
                return

            self.log.debug('inter-adapter-send-onu-ind', onu_indication=onu_indication)

            # TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
            yield self.adapter_proxy.send_inter_adapter_message(
                msg=onu_indication,
                type=InterAdapterMessageType.ONU_IND_REQUEST,
                from_adapter="openolt",
                to_adapter=onu_device.type,
                to_device_id=onu_device.id
            )

        else:
            self.log.warn('Not-implemented-or-invalid-value-of-oper-state',
                          oper_state=onu_indication.oper_state)

    def onu_ports_down(self, onu_device, oper_state):
        # Set port oper state to Discovered
        # add port will update port if it exists
        # self.adapter_agent.add_port(
        #    self.device_id,
        #    Port(
        #        port_no=uni_no,
        #        label=uni_name,
        #        type=Port.ETHERNET_UNI,
        #        admin_state=onu_device.admin_state,
        #        oper_status=oper_state))
        # TODO this should be downning ports in onu adatper

        # Disable logical port
        onu_ports = self.proxy.get('devices/{}/ports'.format(onu_device.id))
        for onu_port in onu_ports:
            self.log.debug('onu-ports-down', onu_port=onu_port)
            onu_port_id = onu_port.label
            try:
                onu_logical_port = self.adapter_agent.get_logical_port(
                    logical_device_id=self.logical_device_id, port_id=onu_port_id)
                onu_logical_port.ofp_port.state = OFPPS_LINK_DOWN
                self.adapter_agent.update_logical_port(
                    logical_device_id=self.logical_device_id,
                    port=onu_logical_port)
                self.log.debug('cascading-oper-state-to-port-and-logical-port')
            except KeyError as e:
                self.log.error('matching-onu-port-label-invalid',
                               onu_id=onu_device.id, olt_id=self.device_id,
                               onu_ports=onu_ports, onu_port_id=onu_port_id,
                               error=e)

    @inlineCallbacks
    def omci_indication(self, omci_indication):

        self.log.debug("omci indication", intf_id=omci_indication.intf_id,
                       onu_id=omci_indication.onu_id)

        onu_device = yield self.core_proxy.get_child_device(
            self.device_id, onu_id=omci_indication.onu_id,
            parent_port_no=self.platform.intf_id_to_port_no(
                omci_indication.intf_id, Port.PON_OLT), )

        omci_msg = InterAdapterOmciMessage(message=omci_indication.pkt)

        self.log.debug('inter-adapter-send-omci', omci_msg=omci_msg)

        # TODO NEW CORE do not hardcode adapter name. Handler needs Adapter reference
        yield self.adapter_proxy.send_inter_adapter_message(
            msg=omci_msg,
            type=InterAdapterMessageType.OMCI_REQUEST,
            from_adapter="openolt",
            to_adapter=onu_device.type,
            to_device_id=onu_device.id
        )

    @inlineCallbacks
    def packet_indication(self, pkt_indication):

        self.log.debug("packet indication",
                       intf_type=pkt_indication.intf_type,
                       intf_id=pkt_indication.intf_id,
                       port_no=pkt_indication.port_no,
                       cookie=pkt_indication.cookie,
                       gemport_id=pkt_indication.gemport_id,
                       flow_id=pkt_indication.flow_id)

        if pkt_indication.intf_type == "pon":
            if pkt_indication.port_no:
                port_num = pkt_indication.port_no
            else:  # TODO Remove this else block after openolt device has been fully rolled out with cookie protobuf change
                try:
                    onu_id_uni_id = self.resource_mgr.get_onu_uni_from_ponport_gemport(pkt_indication.intf_id,
                                                                                       pkt_indication.gemport_id)
                    onu_id = int(onu_id_uni_id[0])
                    uni_id = int(onu_id_uni_id[1])
                    self.log.debug("packet indication-kv", onu_id=onu_id, uni_id=uni_id)
                    if onu_id is None:
                        raise Exception("onu-id-none")
                    if uni_id is None:
                        raise Exception("uni-id-none")
                    port_num = self.platform.mk_uni_port_num(pkt_indication.intf_id, onu_id, uni_id)
                except Exception as e:
                    self.log.error("no-onu-reference-for-gem",
                                   gemport_id=pkt_indication.gemport_id, e=e)
                    return


        elif pkt_indication.intf_type == "nni":
            port_num = self.platform.intf_id_to_port_no(
                pkt_indication.intf_id,
                Port.ETHERNET_NNI)

        pkt = Ether(pkt_indication.pkt)

        self.log.debug("packet indication",
                       device_id=self.device_id,
                       port_num=port_num)

        yield self.core_proxy.send_packet_in(
            device_id=self.device_id,
            port=port_num,
            packet=str(pkt))

    def packet_out(self, egress_port, msg):
        pkt = Ether(msg)
        self.log.debug('packet out', egress_port=egress_port,
                       device_id=self.device_id,
                       logical_device_id=self.logical_device_id,
                       packet=str(pkt).encode("HEX"))

        # Find port type
        egress_port_type = self.platform.intf_id_to_port_type_name(egress_port)
        if egress_port_type == Port.ETHERNET_UNI:

            if pkt.haslayer(Dot1Q):
                outer_shim = pkt.getlayer(Dot1Q)
                if isinstance(outer_shim.payload, Dot1Q):
                    # If double tag, remove the outer tag
                    payload = (
                            Ether(src=pkt.src, dst=pkt.dst, type=outer_shim.type) /
                            outer_shim.payload
                    )
                else:
                    payload = pkt
            else:
                payload = pkt

            send_pkt = binascii.unhexlify(str(payload).encode("HEX"))

            self.log.debug(
                'sending-packet-to-ONU', egress_port=egress_port,
                intf_id=self.platform.intf_id_from_uni_port_num(egress_port),
                onu_id=self.platform.onu_id_from_port_num(egress_port),
                uni_id=self.platform.uni_id_from_port_num(egress_port),
                port_no=egress_port,
                packet=str(payload).encode("HEX"))

            onu_pkt = openolt_pb2.OnuPacket(
                intf_id=self.platform.intf_id_from_uni_port_num(egress_port),
                onu_id=self.platform.onu_id_from_port_num(egress_port),
                port_no=egress_port,
                pkt=send_pkt)

            self.stub.OnuPacketOut(onu_pkt)

        elif egress_port_type == Port.ETHERNET_NNI:
            self.log.debug('sending-packet-to-uplink', egress_port=egress_port,
                           packet=str(pkt).encode("HEX"))

            send_pkt = binascii.unhexlify(str(pkt).encode("HEX"))

            uplink_pkt = openolt_pb2.UplinkPacket(
                intf_id=self.platform.intf_id_from_nni_port_num(egress_port),
                pkt=send_pkt)

            self.stub.UplinkPacketOut(uplink_pkt)

        else:
            self.log.warn('Packet-out-to-this-interface-type-not-implemented',
                          egress_port=egress_port,
                          port_type=egress_port_type)

    @inlineCallbacks
    def process_inter_adapter_message(self, request):
        self.log.debug('process-inter-adapter-message', msg=request)
        try:
            if request.header.type == InterAdapterMessageType.OMCI_REQUEST:
                omci_msg = InterAdapterOmciMessage()
                request.body.Unpack(omci_msg)
                self.log.debug('inter-adapter-recv-omci', omci_msg=omci_msg)

                onu_device_id = request.header.to_device_id
                onu_device = yield self.core_proxy.get_device(onu_device_id)
                self.send_proxied_message(onu_device, omci_msg.message)

            else:
                self.log.error("inter-adapter-unhandled-type", request=request)

        except Exception as e:
            self.log.exception("error-processing-inter-adapter-message", e=e)

    def send_proxied_message(self, onu_device, msg):

        if onu_device.connect_status != ConnectStatus.REACHABLE:
            self.log.debug('ONU is not reachable, cannot send OMCI',
                           serial_number=onu_device.serial_number,
                           intf_id=onu_device.proxy_address.channel_id,
                           onu_id=onu_device.proxy_address.onu_id)
            return

        omci = openolt_pb2.OmciMsg(intf_id=onu_device.proxy_address.channel_id,
                                   onu_id=onu_device.proxy_address.onu_id, pkt=str(msg))
        self.stub.OmciMsgOut(omci)

        self.log.debug("omci-message-sent", intf_id=onu_device.proxy_address.channel_id,
                                   onu_id=onu_device.proxy_address.onu_id, pkt=str(msg))

    @inlineCallbacks
    def add_onu_device(self, intf_id, port_no, onu_id, serial_number):
        self.log.info("adding-onu", port_no=port_no, onu_id=onu_id,
                      serial_number=serial_number)

        serial_number_str = self.stringify_serial_number(serial_number)

        # TODO NEW CORE dont hardcode child device type.  find some way of determining by vendor in serial number
        yield self.core_proxy.child_device_detected(
            parent_device_id=self.device_id,
            parent_port_no=port_no,
            child_device_type='brcm_openomci_onu',
            channel_id=intf_id,
            vendor_id=serial_number.vendor_id,
            serial_number=serial_number_str,
            onu_id=onu_id
        )

        self.log.debug("onu-added", onu_id=onu_id, port_no=port_no, serial_number=serial_number_str)

    def get_ofp_device_info(self, device):
        self.log.info('get_ofp_device_info', device_id=device.id)

        mfr_desc = self.device_info.vendor
        sw_desc = self.device_info.firmware_version
        hw_desc = self.device_info.model
        if self.device_info.hardware_version: hw_desc += '-' + self.device_info.hardware_version

        return SwitchCapability(
            desc=ofp_desc(
                hw_desc=hw_desc,
                sw_desc=sw_desc,
                serial_num=device.serial_number
            ),
            switch_features=ofp_switch_features(
                n_buffers=256,  # Max packets buffered at once          # TODO fake for now
                n_tables=2,  # Number of tables supported by datapath   # TODO fake for now
                capabilities=( #Bitmap of support "ofp_capabilities"    # TODO fake for now
                        OFPC_FLOW_STATS
                        | OFPC_TABLE_STATS
                        | OFPC_PORT_STATS
                        | OFPC_GROUP_STATS
                )
            )
        )

    def get_ofp_port_info(self, device, port_no):
        self.log.info('get_ofp_port_info', port_no=port_no, device_id=device.id)
        cap = OFPPF_1GB_FD | OFPPF_FIBER
        return PortCapability(
            port=LogicalPort(
                ofp_port=ofp_port(
                    hw_addr=mac_str_to_tuple(self._get_mac_form_port_no(port_no)),
                    config=0,
                    state=OFPPS_LIVE,
                    curr=cap,
                    advertised=cap,
                    peer=cap,
                    curr_speed=OFPPF_1GB_FD,
                    max_speed=OFPPF_1GB_FD
                ),
                device_id=device.id,
                device_port_no=port_no
            )
        )

    def port_name(self, port_no, port_type, intf_id=None, serial_number=None):
        if port_type is Port.ETHERNET_NNI:
            return "nni-" + str(port_no)
        elif port_type is Port.PON_OLT:
            return "pon" + str(intf_id)
        elif port_type is Port.ETHERNET_UNI:
            assert False, 'local UNI management not supported'

    def add_logical_port(self, port_no, intf_id, oper_state):
        self.log.info('adding-logical-port', port_no=port_no)

        label = self.port_name(port_no, Port.ETHERNET_NNI)

        cap = OFPPF_1GB_FD | OFPPF_FIBER
        curr_speed = OFPPF_1GB_FD
        max_speed = OFPPF_1GB_FD

        if oper_state == OperStatus.ACTIVE:
            of_oper_state = OFPPS_LIVE
        else:
            of_oper_state = OFPPS_LINK_DOWN

        ofp = ofp_port(
            port_no=port_no,
            hw_addr=mac_str_to_tuple(self._get_mac_form_port_no(port_no)),
            name=label, config=0, state=of_oper_state, curr=cap,
            advertised=cap, peer=cap, curr_speed=curr_speed,
            max_speed=max_speed)

        ofp_stats = ofp_port_stats(port_no=port_no)

        logical_port = LogicalPort(
            id=label, ofp_port=ofp, device_id=self.device_id,
            device_port_no=port_no, root_port=True,
            ofp_port_stats=ofp_stats)

        self.adapter_agent.add_logical_port(self.logical_device_id,
                                            logical_port)

    def _get_mac_form_port_no(self, port_no):
        mac = ''
        for i in range(4):
            mac = ':%02x' % ((port_no >> (i * 8)) & 0xff) + mac
        return '00:00' + mac

    @inlineCallbacks
    def add_port(self, intf_id, port_type, oper_status):
        port_no = self.platform.intf_id_to_port_no(intf_id, port_type)

        label = self.port_name(port_no, port_type, intf_id)

        self.log.debug('adding-port', port_no=port_no, label=label,
                       port_type=port_type)

        port = Port(port_no=port_no, label=label, type=port_type,
                    admin_state=AdminState.ENABLED, oper_status=oper_status)

        yield self.core_proxy.port_created(self.device_id, port)

    def delete_logical_port(self, child_device):
        logical_ports = self.proxy.get('/logical_devices/{}/ports'.format(
            self.logical_device_id))
        for logical_port in logical_ports:
            if logical_port.device_id == child_device.id:
                self.log.debug('delete-logical-port',
                               onu_device_id=child_device.id,
                               logical_port=logical_port)
                self.flow_mgr.clear_flows_and_scheduler_for_logical_port(
                    child_device, logical_port)
                self.adapter_agent.delete_logical_port(
                    self.logical_device_id, logical_port)
                return

    @inlineCallbacks
    def delete_port(self, child_serial_number):
        ports = self.proxy.get('/devices/{}/ports'.format(
            self.device_id))
        for port in ports:
            if port.label == child_serial_number:
                self.log.debug('delete-port',
                               onu_serial_number=child_serial_number,
                               port=port)
                yield self.adapter_agent.delete_port(self.device_id, port)
                return

    def update_flow_table(self, flow_changes):

        self.log.debug("update_flow_table", flow_changes=flow_changes)

        flows_to_add = flow_changes.to_add.items
        flows_to_remove = flow_changes.to_remove.items

        if not self.is_state_up():
            self.log.info('The OLT is not up, we cannot update flows',
                          flows_to_add=[f.id for f in flows_to_add],
                          flows_to_remove=[f.id for f in flows_to_remove])
            return

        self.log.debug('logical flows update', flows_to_add=flows_to_add,
                       flows_to_remove=flows_to_remove)

        for flow in flows_to_add:

            try:
                self.flow_mgr.add_flow(flow)
            except Exception as e:
                self.log.error('failed to add flow', flow=flow, e=e)

        for flow in flows_to_remove:

            try:
                self.flow_mgr.remove_flow(flow)
            except Exception as e:
                self.log.error('failed to remove flow', flow=flow, e=e)

        # TODO NEW CORE: Core keeps track of logical flows. no need to keep track.  verify, especially olt reboot!
        #self.flow_mgr.repush_all_different_flows()

    # There has to be a better way to do this
    def ip_hex(self, ip):
        octets = ip.split(".")
        hex_ip = []
        for octet in octets:
            octet_hex = hex(int(octet))
            octet_hex = octet_hex.split('0x')[1]
            octet_hex = octet_hex.rjust(2, '0')
            hex_ip.append(octet_hex)
        return ":".join(hex_ip)

    def stringify_vendor_specific(self, vendor_specific):
        return ''.join(str(i) for i in [
            hex(ord(vendor_specific[0]) >> 4 & 0x0f)[2:],
            hex(ord(vendor_specific[0]) & 0x0f)[2:],
            hex(ord(vendor_specific[1]) >> 4 & 0x0f)[2:],
            hex(ord(vendor_specific[1]) & 0x0f)[2:],
            hex(ord(vendor_specific[2]) >> 4 & 0x0f)[2:],
            hex(ord(vendor_specific[2]) & 0x0f)[2:],
            hex(ord(vendor_specific[3]) >> 4 & 0x0f)[2:],
            hex(ord(vendor_specific[3]) & 0x0f)[2:]])

    def stringify_serial_number(self, serial_number):
        return ''.join([serial_number.vendor_id,
                        self.stringify_vendor_specific(
                            serial_number.vendor_specific)])

    def destringify_serial_number(self, serial_number_str):
        serial_number = openolt_pb2.SerialNumber(
            vendor_id=serial_number_str[:4].encode('utf-8'),
            vendor_specific=binascii.unhexlify(serial_number_str[4:]))
        return serial_number

    def disable(self):
        self.log.debug('sending-deactivate-olt-message',
                       device_id=self.device_id)

        try:
            # Send grpc call
            self.stub.DisableOlt(openolt_pb2.Empty())
            # The resulting indication will bring the OLT down
            # self.go_state_down()
            self.log.info('openolt device disabled')
        except Exception as e:
            self.log.error('Failure to disable openolt device', error=e)

    def delete(self):
        self.log.info('deleting-olt', device_id=self.device_id,
                      logical_device_id=self.logical_device_id)

        # Clears up the data from the resource manager KV store
        # for the device
        del self.resource_mgr

        try:
            # Rebooting to reset the state
            self.reboot()
            # Removing logical device
        except Exception as e:
            self.log.error('Failure to delete openolt device', error=e)
            raise e
        else:
            self.log.info('successfully-deleted-olt', device_id=self.device_id)

    def reenable(self):
        self.log.debug('reenabling-olt', device_id=self.device_id)

        try:
            self.stub.ReenableOlt(openolt_pb2.Empty())

        except Exception as e:
            self.log.error('Failure to reenable openolt device', error=e)
        else:
            self.log.info('openolt device reenabled')

    def activate_onu(self, intf_id, onu_id, serial_number,
                     serial_number_str):
        pir = self.bw_mgr.pir(serial_number_str)
        self.log.debug("activating-onu", intf_id=intf_id, onu_id=onu_id,
                       serial_number_str=serial_number_str,
                       serial_number=serial_number, pir=pir)
        onu = openolt_pb2.Onu(intf_id=intf_id, onu_id=onu_id,
                              serial_number=serial_number, pir=pir)
        self.stub.ActivateOnu(onu)
        self.log.info('onu-activated', serial_number=serial_number_str)

    @inlineCallbacks
    def delete_child_device(self, child_device):
        self.log.debug('sending-deactivate-onu',
                       olt_device_id=self.device_id,
                       onu_device=child_device,
                       onu_serial_number=child_device.serial_number)
        try:
            yield self.adapter_agent.delete_child_device(self.device_id,
                                                   child_device.id,
                                                   child_device)
        except Exception as e:
            self.log.error('adapter_agent error', error=e)
        try:
            self.delete_logical_port(child_device)
        except Exception as e:
            self.log.error('logical_port delete error', error=e)
        try:
            self.delete_port(child_device.serial_number)
        except Exception as e:
            self.log.error('port delete error', error=e)
        serial_number = self.destringify_serial_number(
            child_device.serial_number)
        # TODO FIXME - For each uni.
        # TODO FIXME - Flows are not deleted
        uni_id = 0  # FIXME
        self.flow_mgr.delete_tech_profile_instance(
            child_device.proxy_address.channel_id,
            child_device.proxy_address.onu_id,
            uni_id
        )
        pon_intf_id_onu_id = (child_device.proxy_address.channel_id,
                              child_device.proxy_address.onu_id,
                              uni_id)
        # Free any PON resources that were reserved for the ONU
        self.resource_mgr.free_pon_resources_for_onu(pon_intf_id_onu_id)

        onu = openolt_pb2.Onu(intf_id=child_device.proxy_address.channel_id,
                              onu_id=child_device.proxy_address.onu_id,
                              serial_number=serial_number)
        self.stub.DeleteOnu(onu)

    def reboot(self):
        self.log.debug('rebooting openolt device', device_id=self.device_id)
        try:
            self.stub.Reboot(openolt_pb2.Empty())
        except Exception as e:
            self.log.error('something went wrong with the reboot', error=e)
        else:
            self.log.info('device rebooted')

    def trigger_statistics_collection(self):
        try:
            self.stub.CollectStatistics(openolt_pb2.Empty())
        except Exception as e:
            self.log.error('Error while triggering statistics collection',
                           error=e)
        else:
            self.log.info('statistics requested')

    def simulate_alarm(self, alarm):
        self.alarm_mgr.simulate_alarm(alarm)
