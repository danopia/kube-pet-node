package selfprovision

import (
	"fmt"
	"net"

	ini "gopkg.in/ini.v1"
)

// ClusterConfig !
type ClusterConfig struct {
	NodeRange    *net.IPNet
	PodRange     *net.IPNet
	ServiceRange *net.IPNet

	WireguardMode string
}

// ParseClusterCfg !
func ParseClusterCfg(data string) (*ClusterConfig, error) {

	file, err := ini.Load([]byte(data))
	if err != nil {
		return nil, err
	}

	var clusterCfg ClusterConfig

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

	clusterCfg.WireguardMode = file.Section("DEFAULT").Key("WireguardMode").String()

	return &clusterCfg, nil
}

// ClusterCniConfig !
type ClusterCniConfig struct {
	CniNumber int
	CniMtu    int
}

// ParseClusterCniCfg !
func ParseClusterCniCfg(data string) (*ClusterCniConfig, error) {

	file, err := ini.Load([]byte(data))
	if err != nil {
		return nil, err
	}

	var cniConfig ClusterCniConfig
	if err := file.Section("DEFAULT").MapTo(&cniConfig); err != nil {
		return nil, err
	}

	return &cniConfig, nil
}
