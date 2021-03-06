package machine

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"
	"github.com/coreos/fleet/third_party/github.com/dotcloud/docker/pkg/netlink"
)

const bootIdPath = "/proc/sys/kernel/random/boot_id"

// MachineState represents a point-in-time snapshot of the
// state of the local host.
type MachineState struct {
	BootId   string
	PublicIP string
	Metadata map[string]string
}

func (ms MachineState) String() string {
	return fmt.Sprintf("MachineState{BootId: %q, PublicIp: %q, Metadata: %v}", ms.BootId, ms.PublicIP, ms.Metadata)
}

// NewDynamicMachineState generates a MachineState object with
// the values read from the local system
func CurrentState() MachineState {
	bootId := readLocalBootId()
	publicIP := getLocalIP()
	return MachineState{bootId, publicIP, make(map[string]string, 0)}
}

func readLocalBootId() string {
	id, err := ioutil.ReadFile(bootIdPath)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(id))
}

func getLocalIP() string {
	iface := getDefaultGatewayIface()
	if iface == nil {
		return ""
	}

	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return ""
	}

	for _, addr := range addrs {
		// Attempt to parse the address in CIDR notation
		// and assert it is IPv4
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.To4() != nil {
			return ip.String()
		}
	}

	return ""
}

func getDefaultGatewayIface() *net.Interface {
	log.V(2).Infof("Attempting to retrieve IP route info from netlink")

	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		log.V(2).Infof("Unable to detect default interface: %v", err)
		return nil
	}

	if len(routes) == 0 {
		log.V(2).Infof("Netlink returned zero routes")
		return nil
	}

	for _, route := range routes {
		if route.Default {
			if route.Iface == nil {
				log.V(2).Infof("Found default route but could not determine interface")
			}
			log.V(2).Infof("Found default route with interface %v", route.Iface.Name)
			return route.Iface
		}
	}

	log.V(2).Infof("Unable to find default route")
	return nil
}

func stackState(top, bottom MachineState) MachineState {
	state := MachineState(bottom)

	if top.PublicIP != "" {
		state.PublicIP = top.PublicIP
	}

	if top.BootId != "" {
		state.BootId = top.BootId
	}

	//FIXME: This will *always* overwrite the bottom's metadata,
	// but the only use-case we have today does not ever have
	// metadata on the bottom.
	if len(top.Metadata) > 0 {
		state.Metadata = top.Metadata
	}

	return state
}
