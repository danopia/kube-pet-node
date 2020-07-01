package podman

import "net"

// NetOptions reflect the shared network options between
// pods and containers
type NetOptions struct {
	AddHosts           []string
	CNINetworks        []string
	UseImageResolvConf bool
	DNSOptions         []string
	DNSSearch          []string
	DNSServers         []net.IP
	Network            Namespace
	NoHosts            bool
	PublishPorts       []PortMapping
	StaticIP           *net.IP
	StaticMAC          *net.HardwareAddr
}
