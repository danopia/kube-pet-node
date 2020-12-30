package selfprovision

import (
	"fmt"
	"net"

	ini "gopkg.in/ini.v1"
)

// ClusterNetworkingConfig !
type ClusterNetworkingConfig struct {
	NodeRange    *net.IPNet
	PodRange     *net.IPNet
	ServiceRange *net.IPNet

	WireguardMode string

	CniNumber int
	CniMtu    int

	// also routers, not relevant here
}

// ParseClusterNetworkingCfg !
func ParseClusterNetworkingCfg(data string) (*ClusterNetworkingConfig, error) {

	file, err := ini.Load([]byte(data))
	if err != nil {
		return nil, err
	}

	var clusterCfg ClusterNetworkingConfig

	_, clusterCfg.NodeRange, err = net.ParseCIDR(file.Section("DEFAULT").Key("NodeRange").String())
	if err != nil {
		return nil, fmt.Errorf("Parsing NodeRange: %w", err)
	}
	_, clusterCfg.PodRange, err = net.ParseCIDR(file.Section("DEFAULT").Key("PodRange").String())
	if err != nil {
		return nil, fmt.Errorf("Parsing PodRange: %w", err)
	}
	_, clusterCfg.ServiceRange, err = net.ParseCIDR(file.Section("DEFAULT").Key("ServiceRange").String())
	if err != nil {
		return nil, fmt.Errorf("Parsing ServiceRange: %w", err)
	}

	clusterCfg.WireguardMode = file.Section("DEFAULT").Key("WireguardMode").MustString("Manual")

	clusterCfg.CniNumber = file.Section("DEFAULT").Key("CniNumber").MustInt(51)
	clusterCfg.CniMtu = file.Section("DEFAULT").Key("CniMtu").MustInt(1460)

	return &clusterCfg, nil
}
