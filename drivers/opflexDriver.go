package drivers

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils/netutils"
)

// OpflexDriverConfig represents the configuration of the opflex elements,
type OpflexDriverConfig struct {
	Opflex struct {
		EpFilePath string
		VmmDomain  string
	}
}

// OpflexDriverOperState carries operational state of the OpflexDriver.
type OpflexDriverOperState struct {
	core.CommonState
	// used to allocate port names. XXX: should it be user controlled?
	CurrPortNum int `json:"currPortNum"`
}

// Write the state
func (s *OpflexDriverOperState) Write() error {
	key := fmt.Sprintf(ovsOperPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state given an ID.
func (s *OpflexDriverOperState) Read(id string) error {
	key := fmt.Sprintf(ovsOperPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all the state
func (s *OpflexDriverOperState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(ovsOperPathPrefix, s, json.Unmarshal)
}

// Clear removes the state.
func (s *OpflexDriverOperState) Clear() error {
	key := fmt.Sprintf(ovsOperPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// OpflexDriver implements core.NetworkDriver interface
// for use with opflex integration
type OpflexDriver struct {
	oper OpflexDriverOperState // Oper state of the driver
}

func (d *OpflexDriver) getIntfName() (string, error) {
	// get the next available port number
	for i := 0; i < maxIntfRetry; i++ {
		// Pick next port number
		d.oper.CurrPortNum++
		intfName := fmt.Sprintf("vport%d", d.oper.CurrPortNum)

		// check if the port name is already in use
		_, err := netlink.LinkByName(intfName)
		if err != nil && strings.Contains(err.Error(), "not found") {
			// save the new state
			err = d.oper.Write()
			if err != nil {
				return "", err
			}
			return intfName, nil
		}
	}

	return "", core.Errorf("Could not get intf name. Max retry exceeded")
}

// Init is not implemented.
func (d *OpflexDriver) Init(config *core.Config, info *core.InstanceInfo) error {
	log.Infof("Opflex Driver: Init")

	if config == nil || info == nil || info.StateDriver == nil {
		return core.Errorf("Invalid arguments. cfg: %+v, instance-info: %+v",
			config, info)
	}

	_, ok := config.V.(*OpflexDriverConfig)
	if !ok {
		return core.Errorf("Invalid type passed")
	}

	d.oper.StateDriver = info.StateDriver

	// restore the driver's runtime state if it exists
	err := d.oper.Read(info.HostLabel)
	if core.ErrIfKeyExists(err) != nil {
		log.Printf("Failed to read driver oper state for key %q. Error: %s",
			info.HostLabel, err)
		return err
	} else if err != nil {
		// create the oper state as it is first time start up
		d.oper.ID = info.HostLabel
		err = d.oper.Write()
		if err != nil {
			return err
		}
	}

	log.Infof("Opflex: Initializing opflex driver")

	return nil
}

// Deinit is not implemented.
func (d *OpflexDriver) Deinit() {
	log.Infof("Opflex Driver: Deinit")
}

// CreateNetwork creates a network in opflex
func (d *OpflexDriver) CreateNetwork(id string) error {
	log.Infof("Opflex Driver: create network %s", id)
	return nil
}

// DeleteNetwork deletes the network in opflex
func (d *OpflexDriver) DeleteNetwork(id, encap string, pktTag, extPktTag int, gateway string) error {
	log.Infof("Opflex Driver: delete network %s", id)
	return nil
}

// CreatePort creates the port for opflex
func (d *OpflexDriver) CreatePort(intfName string, cfgEp *mastercfg.CfgEndpointState, pktTag, nwPktTag int) error {
	var ovsIntfType string

	// Get OVS port name
	ovsPortName := getExtPortName(intfName)

	// Create Veth pairs if required
	if useVethPair {
		ovsIntfType = ""

		// Create a Veth pair
		err := createVethPair(intfName, ovsPortName)
		if err != nil {
			log.Errorf("Error creating veth pairs. Err: %v", err)
			return err
		}

		// Set the OVS side of the port as up
		err = setLinkUp(ovsPortName)
		if err != nil {
			log.Errorf("Error setting link %s up. Err: %v", ovsPortName, err)
			return err
		}
	} else {
		ovsPortName = intfName
		ovsIntfType = "internal"

	}

	// Set the link mtu to 1450 to allow for 50 bytes vxlan encap
	// (inner eth header(14) + outer IP(20) outer UDP(8) + vxlan header(8))
	err := setLinkMtu(intfName, vxlanEndpointMtu)
	if err != nil {
		log.Errorf("Error setting link %s mtu. Err: %v", intfName, err)
		return err
	}

	err = nil
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			log.Infof("TODO Opflex: create port failed: delete name %s type %s id %s tag %d", ovsPortName, ovsIntfType, cfgEp.ID, pktTag)
			log.Infof("TODO Opflex: delete port ", intfName)
		}
	}()

	// Wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	// Set the interface mac address
	err = netutils.SetInterfaceMac(intfName, cfgEp.MacAddress)
	if err != nil {
		log.Errorf("Error setting interface Mac %s on port %s", cfgEp.MacAddress, intfName)
		return err
	}

	macAddr, _ := net.ParseMAC(cfgEp.MacAddress)
	log.Infof("TODO Opflex: create port: name %s type %s id %s tag %d", ovsPortName, ovsIntfType, cfgEp.ID, pktTag)
	log.Infof("TODO Opflex: create port: nwPktTag %s, macAddr %s, ip %s, epgid %s", nwPktTag, macAddr, cfgEp.IPAddress, cfgEp.EndpointGroupID)

	// Add the local port to ofnet
	return nil
}

// CreateEndpoint is not implemented.
func (d *OpflexDriver) CreateEndpoint(id string) error {
	log.Infof("Opflex Driver: create endpoint %s", id)
	var (
		err      error
		intfName string
	)

	cfgEp := &mastercfg.CfgEndpointState{}
	cfgEp.StateDriver = d.oper.StateDriver
	err = cfgEp.Read(id)
	if err != nil {
		return err
	}

	// Get the nw config.
	cfgNw := mastercfg.CfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err = cfgNw.Read(cfgEp.NetID)
	if err != nil {
		log.Errorf("Unable to get network %s. Err: %v", cfgEp.NetID, err)
		return err
	}

	cfgEpGroup := &mastercfg.EndpointGroupState{}
	cfgEpGroup.StateDriver = d.oper.StateDriver
	err = cfgEpGroup.Read(strconv.Itoa(cfgEp.EndpointGroupID))
	if err == nil {
		log.Debugf("pktTag: %v ", cfgEpGroup.PktTag)
	} else if core.ErrIfKeyExists(err) == nil {
		log.Infof("%v will use network based tag ", err)
		cfgEpGroup.PktTagType = cfgNw.PktTagType
		cfgEpGroup.PktTag = cfgNw.PktTag
	} else {
		return err
	}

	operEp := &OvsOperEndpointState{}
	operEp.StateDriver = d.oper.StateDriver
	err = operEp.Read(id)
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err == nil {
		// check if oper state matches cfg state. In case of mismatch cleanup
		// up the EP and continue add new one. In case of match just return.
		if operEp.Matches(cfgEp) {
			log.Printf("Found matching oper state for ep %s, noop", id)

			// Ask the switch to update the port
			log.Infof("Opflex TODO: Updating port %s. Err: %v", intfName, err)
			err = nil
			if err != nil {
				log.Errorf("Error updating port %s. Err: %v", intfName, err)
				return err
			}

			return nil
		}
		log.Printf("Found mismatching oper state for Ep, cleaning it. Config: %+v, Oper: %+v",
			cfgEp, operEp)
		d.DeleteEndpoint(operEp.ID)
	}

	// Get the interface name to use
	intfName, err = d.getIntfName()
	if err != nil {
		return err
	}

	// Ask the switch to create the port
	err = d.CreatePort(intfName, cfgEp, cfgEpGroup.PktTag, cfgNw.PktTag)
	if err != nil {
		log.Errorf("Error creating port %s. Err: %v", intfName, err)
		return err
	}

	// Save the oper state
	operEp = &OvsOperEndpointState{
		NetID:       cfgEp.NetID,
		AttachUUID:  cfgEp.AttachUUID,
		ContName:    cfgEp.ContName,
		ServiceName: cfgEp.ServiceName,
		IPAddress:   cfgEp.IPAddress,
		MacAddress:  cfgEp.MacAddress,
		IntfName:    cfgEp.IntfName,
		PortName:    intfName,
		HomingHost:  cfgEp.HomingHost,
		VtepIP:      cfgEp.VtepIP}
	operEp.StateDriver = d.oper.StateDriver
	operEp.ID = id
	err = operEp.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			operEp.Clear()
		}
	}()

	return nil
}

// DeletePort removes a port from OVS
func (d *OpflexDriver) DeletePort(epOper *OvsOperEndpointState) error {

	if epOper.VtepIP != "" {
		return nil
	}

	// Get the OVS port name
	ovsPortName := getExtPortName(epOper.PortName)
	if !useVethPair {
		ovsPortName = epOper.PortName
	}

	// Delete it from opflex driver
	log.Infof("TODO Opflex: deleting port ", ovsPortName)

	var err error
	err = nil
	if err != nil {
		log.Errorf("Error deleting port %s from OVS. Err: %v", ovsPortName, err)
		// continue with further cleanup
	}

	// Delete the Veth pairs if required
	if useVethPair {
		// Delete a Veth pair
		verr := deleteVethPair(ovsPortName, epOper.PortName)
		if verr != nil {
			log.Errorf("Error creating veth pairs. Err: %v", verr)
			return verr
		}
	}

	return err
}

// DeleteEndpoint is not implemented.
func (d *OpflexDriver) DeleteEndpoint(id string) (err error) {
	log.Infof("Opflex Driver: delete endpoint %s", id)

	epOper := OvsOperEndpointState{}
	epOper.StateDriver = d.oper.StateDriver
	err = epOper.Read(id)
	if err != nil {
		return err
	}
	defer func() {
		epOper.Clear()
	}()

	// Get the network state
	cfgNw := mastercfg.CfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err = cfgNw.Read(epOper.NetID)
	if err != nil {
		return err
	}

	err = d.DeletePort(&epOper)
	if err != nil {
		log.Errorf("Error deleting endpoint: %+v. Err: %v", epOper, err)
	}

	return nil
}

// AddPeerHost is not implemented.
func (d *OpflexDriver) AddPeerHost(node core.ServiceInfo) error {
	log.Infof("Opflex Driver: add peer host %#v", node)
	return nil
}

// DeletePeerHost is not implemented.
func (d *OpflexDriver) DeletePeerHost(node core.ServiceInfo) error {
	log.Infof("Opflex Driver: delete peer host %#v", node)
	return nil
}

// AddMaster is not implemented
func (d *OpflexDriver) AddMaster(node core.ServiceInfo) error {
	log.Infof("Opflex Driver: add master %#v", node)
	return nil
}

// DeleteMaster is not implemented
func (d *OpflexDriver) DeleteMaster(node core.ServiceInfo) error {
	log.Infof("Opflex Driver: delete master %#v", node)
	return nil
}

// AddBgp is not implemented.
func (d *OpflexDriver) AddBgp(id string) (err error) {
	log.Infof("Opflex Driver: add bgp %s", id)
	return nil
}

// DeleteBgp is not implemented.
func (d *OpflexDriver) DeleteBgp(id string) (err error) {
	log.Infof("Opflex Driver: delete bgp %s", id)
	return nil
}

// AddSvcSpec is not implemented.
func (d *OpflexDriver) AddSvcSpec(svcName string, spec *core.ServiceSpec) error {
	log.Infof("Opflex Driver: add svc %s %#v", svcName, spec)
	return nil
}

// DelSvcSpec is not implemented.
func (d *OpflexDriver) DelSvcSpec(svcName string, spec *core.ServiceSpec) error {
	log.Infof("Opflex Driver: delete svc %s %#v", svcName, spec)
	return nil
}

// SvcProviderUpdate is not implemented.
func (d *OpflexDriver) SvcProviderUpdate(svcName string, providers []string) {
	log.Infof("Opflex Driver: update svc %s %#v", svcName, providers)
}
