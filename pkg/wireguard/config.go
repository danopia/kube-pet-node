package wireguard

import (
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"gopkg.in/ini.v1"
)

// ByName returns a handle to manage a specific WireGuard interface.
func ByName(ifaceName string) *WgInterface {
	return &WgInterface{ifaceName}
}

type WgInterface struct {
	name string
}

func (wg *WgInterface) ReadPersistentConfig() (*WgQuickConfig, error) {
	raw, err := ioutil.ReadFile("/etc/wireguard/" + wg.name + ".conf")
	// raw, err := ioutil.ReadFile("/tmp/" + wg.name + ".conf")
	if err != nil {
		return nil, err
	}

	return ParseWgQuickConfig(string(raw))
}

func (wg *WgInterface) WritePersistentConfig(config *WgQuickConfig) error {
	configRaw := []byte(config.Stringify())

	return ioutil.WriteFile("/etc/wireguard/"+wg.name+".conf", configRaw, 0660)
	// return ioutil.WriteFile("/tmp/"+wg.name+".conf", configRaw, 0660)
}

type WgQuickConfig struct {
	Comment      string
	PrivateKey   string
	Addresses    []*net.IPNet
	ListenPort   int
	FirewallMark int
	Peers        []PeerConfig
}
type PeerConfig struct {
	Comment             string
	PublicKey           string
	PresharedKey        string
	Endpoint            string
	PersistentKeepalive int
	AllowedIPs          []*net.IPNet
}

func (cfg *WgQuickConfig) Stringify() string {
	var b strings.Builder

	b.WriteString("[Interface]")
	if cfg.Comment != "" {
		fmt.Fprintf(&b, " # %s", cfg.Comment)
	}
	b.WriteRune('\n')

	if cfg.PrivateKey != "" {
		fmt.Fprintf(&b, "PrivateKey = %s\n", cfg.PrivateKey)
	}
	for _, addr := range cfg.Addresses {
		fmt.Fprintf(&b, "Address = %s\n", addr.String())
	}
	if cfg.ListenPort >= 0 {
		fmt.Fprintf(&b, "ListenPort = %d\n", cfg.ListenPort)
	}
	if cfg.FirewallMark >= 0 {
		fmt.Fprintf(&b, "FirewallMark = %d\n", cfg.FirewallMark)
	}

	for _, peerCfg := range cfg.Peers {
		b.WriteRune('\n')

		b.WriteString("[Peer]")
		if peerCfg.Comment != "" {
			fmt.Fprintf(&b, " # %s", peerCfg.Comment)
		}
		b.WriteRune('\n')

		if peerCfg.PublicKey != "" {
			fmt.Fprintf(&b, "PublicKey = %s\n", peerCfg.PublicKey)
		}
		if peerCfg.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", peerCfg.PresharedKey)
		}
		for _, addr := range peerCfg.AllowedIPs {
			fmt.Fprintf(&b, "AllowedIPs = %s\n", addr.String())
		}
		if peerCfg.Endpoint != "" {
			fmt.Fprintf(&b, "Endpoint = %s\n", peerCfg.Endpoint)
		}
		if peerCfg.PersistentKeepalive >= 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", peerCfg.PersistentKeepalive)
		}
	}

	return b.String()
}

func ParseWgQuickConfig(data string) (*WgQuickConfig, error) {

	file, err := ini.LoadSources(ini.LoadOptions{
		AllowNonUniqueSections: true,
		AllowShadows:           true,
	}, []byte(data))
	if err != nil {
		return nil, err
	}

	interfaceSection := file.Section("Interface")
	peerSections, _ := file.SectionsByName("Peer")

	var networkCfg WgQuickConfig
	if len(interfaceSection.Comment) > 2 {
		networkCfg.Comment = interfaceSection.Comment[2:]
	}
	networkCfg.PrivateKey = interfaceSection.Key("PrivateKey").String()
	networkCfg.Addresses, err = parseNetworkList(interfaceSection.Key("Address"))
	if err != nil {
		return nil, err
	}
	networkCfg.ListenPort = interfaceSection.Key("ListenPort").MustInt(-1)
	networkCfg.FirewallMark = interfaceSection.Key("FirewallMark").MustInt(-1)

	for _, peerSection := range peerSections {
		var peerCfg PeerConfig
		if len(peerSection.Comment) > 2 {
			peerCfg.Comment = peerSection.Comment[2:]
		}
		peerCfg.PublicKey = peerSection.Key("PublicKey").String()
		peerCfg.PresharedKey = peerSection.Key("PresharedKey").String()
		peerCfg.Endpoint = peerSection.Key("Endpoint").String()
		peerCfg.PersistentKeepalive = peerSection.Key("PersistentKeepalive").MustInt(-1)
		peerCfg.AllowedIPs, err = parseNetworkList(peerSection.Key("AllowedIPs"))
		if err != nil {
			return nil, err
		}

		networkCfg.Peers = append(networkCfg.Peers, peerCfg)
	}

	return &networkCfg, nil
}

func parseNetworkList(key *ini.Key) ([]*net.IPNet, error) {
	netList := []*net.IPNet{}
	for _, value := range key.StringsWithShadows(",") {
		if len(value) < 2 {
			continue
		}

		_, ipNet, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("Parsing %v %v: %w", key.Name(), value, err)
		}
		netList = append(netList, ipNet)
	}

	return netList, nil
}
