package podman

import (
	"context"
)

// NetworkCreate(ctx context.Context, name string, options NetworkCreateOptions) (*NetworkCreateReport, error)

// NetworkInspect(ctx context.Context, namesOrIds []string, options NetworkInspectOptions) ([]NetworkInspectReport, error)
func (pc *PodmanClient) NetworkInspect(ctx context.Context, nameOrId string) ([]*NetworkInspectReport, error) {
	encoded, err := UrlEncoded(nameOrId)
	if err != nil {
		return nil, err
	}

	var out []*NetworkInspectReport
	return out, pc.performGet(ctx, "/libpod/networks/"+encoded+"/json", &out)
}

type NetworkInspectReport struct {
	Name         string
	CNIVersion   string
	DisableCheck bool
	Plugins      []NetworkPluginInspect
}
type NetworkPluginInspect struct {
	Type string
	Ipam *NetworkIpamInspect
}
type NetworkIpamInspect struct {
	Type   string
	Subnet string
	Ranges [][]NetworkIpamRange
	Routes []NetworkIpamRoute
}
type NetworkIpamRange struct {
	Gateway    string
	Subnet     string
	RangeStart string
	RangeEnd   string
}
type NetworkIpamRoute struct {
	Dst string
	Gw  string
}

// NetworkList(ctx context.Context, options NetworkListOptions) ([]*NetworkListReport, error)
func (pc *PodmanClient) NetworkList(ctx context.Context) ([]*NetworkConfigList, error) {
	var out []*NetworkConfigList
	return out, pc.performGet(ctx, "/libpod/networks/json", &out)
}

type NetworkConfigList struct {
	Name         string
	CNIVersion   string
	DisableCheck bool
	Plugins      []*NetworkConfig
	Bytes        []byte
}
type NetworkConfig struct {
	Network map[string]interface{}
	Bytes   []byte
}

// NetworkRm(ctx context.Context, namesOrIds []string, options NetworkRmOptions) ([]*NetworkRmReport, error)
