package firewall

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/google/nftables"
)

type byNamespaceAndName []*corev1.Service

func (a byNamespaceAndName) Len() int { return len(a) }
func (a byNamespaceAndName) Less(i, j int) bool {
	iKey := a[i].ObjectMeta.Namespace + "/" + a[i].ObjectMeta.Name
	jKey := a[j].ObjectMeta.Namespace + "/" + a[j].ObjectMeta.Name
	return iKey < jKey
}
func (a byNamespaceAndName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (fc *FirewallController) BuildConfig(nft *NftWriter) error {

	endpoints, err := fc.EndpointsInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}
	endpointMap := make(map[string]*corev1.Endpoints, len(endpoints))
	for _, ep := range endpoints {
		key := ep.ObjectMeta.Namespace + "/" + ep.ObjectMeta.Name
		endpointMap[key] = ep
	}

	services, err := fc.ServiceInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}
	sort.Sort(byNamespaceAndName(services))

	nft.StartTableReplacement("kube-pet", nftables.TableFamilyIPv4)
	rule := NewRuleBuilder()
	podIpsServingClusterIps := make(map[string]string)

	for _, svc := range services {
		key := svc.ObjectMeta.Namespace + "/" + svc.ObjectMeta.Name
		ep, ok := endpointMap[key]
		if !ok || svc.Spec.Type == "ExternalName" {
			continue
		}
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		var healthyAddrs []string
		for _, subset := range ep.Subsets {
			for _, subAddr := range subset.Addresses {
				healthyAddrs = append(healthyAddrs, subAddr.IP)
				// Keep track of pods offering services on the local node
				if subAddr.NodeName != nil {
					if *subAddr.NodeName == fc.NodeName {
						podIpsServingClusterIps[subAddr.IP] = key
					}
				}
			}
		}

		nft.StartBasicChain("svc-" + string(svc.ObjectMeta.UID) + "-dnat")

		for _, port := range svc.Spec.Ports {
			portName := port.Name
			if portName == "" {
				portName = fmt.Sprintf("%v", port.Port)
			}

			var tgtAddrs []string
			var tgtPorts []uint16
			for _, subset := range ep.Subsets {
				samePort := false
				var tgtPort uint16
				for _, subPort := range subset.Ports {
					if subPort.Name == port.Name {
						samePort = true
						tgtPort = uint16(subPort.Port)
					}
				}
				if samePort {
					for _, subAddr := range subset.Addresses {
						tgtAddrs = append(tgtAddrs, subAddr.IP)
						tgtPorts = append(tgtPorts, tgtPort)
					}
				}
			}

			// log.Println("Firewall:", key, svc.ObjectMeta.UID, svc.Spec.ClusterIP, ep.Subsets)

			if len(tgtAddrs) > 0 {

				rule.Reset()
				switch port.Protocol {
				case "TCP":
					rule.IsTCP()
				case "UDP":
					rule.IsUDP()
				case "SCTP":
					rule.IsSCTP()
				}
				rule.IsDestPort(uint16(port.Port))

				// TODO: just uses first addr for now
				rule.Counter()
				rule.TranslateIPv4Destination(tgtAddrs[0], tgtPorts[0])

				nft.AddRuleWithComment(rule, fmt.Sprintf("%v:%v", key, portName))
			}
		}

		// Route ICMP traffic to anything that's at all healthy

		// TODO: just uses first addr for now
		if len(healthyAddrs) > 0 {
			rule.Reset()
			rule.IsICMP()
			rule.Counter()
			rule.TranslateIPv4Address(healthyAddrs[0])
			nft.AddRuleWithComment(rule, key+" ICMP")
		}

		nft.EndChain()

		// nft.StartBasicChain("svc-" + string(svc.ObjectMeta.UID) + "-filter")

		// for _, port := range svc.Spec.Ports {

		// 	var tgtAddrs []string
		// 	var tgtPorts []uint16
		// 	for _, subset := range ep.Subsets {
		// 		samePort := false
		// 		var tgtPort uint16
		// 		for _, subPort := range subset.Ports {
		// 			if subPort.Name == port.Name {
		// 				samePort = true
		// 				tgtPort = uint16(subPort.Port)
		// 			}
		// 		}
		// 		if samePort {
		// 			for _, subAddr := range subset.Addresses {
		// 				tgtAddrs = append(tgtAddrs, subAddr.IP)
		// 				tgtPorts = append(tgtPorts, tgtPort)
		// 			}
		// 		}
		// 	}

		// 	// log.Println("Firewall:", key, svc.ObjectMeta.UID, svc.Spec.ClusterIP, ep.Subsets)

		// 	rule.Reset()
		// 	switch port.Protocol {
		// 	case "TCP":
		// 		rule.IsTCP()
		// 	case "UDP":
		// 		rule.IsUDP()
		// 	case "SCTP":
		// 		rule.IsSCTP()
		// 	}
		// 	rule.IsDestPort(uint16(port.Port))

		// 	rule.Counter()

		// 	if len(tgtAddrs) == 0 {
		// 		rule.RejectAsHostUnreachable()
		// 	} else {
		// 		rule.Accept()
		// 	}

		// 	portName := port.Name
		// 	if portName == "" {
		// 		portName = "default"
		// 	}
		// 	nft.AddRuleWithComment(rule, fmt.Sprintf("%v %v %v", key, port.Protocol, portName))
		// }

		// // Route ICMP traffic to anything that's at all healthy

		// rule.Reset()
		// rule.IsICMP()
		// rule.Counter()
		// if len(healthyAddrs) == 0 {
		// 	rule.RejectAsHostUnreachable()
		// } else {
		// 	rule.Accept()
		// }
		// nft.AddRuleWithComment(rule, key+" ICMP")

		// // Reject all other (unknown port) traffic
		// nft.AddRule(rule.Counter().RejectAsPortUnreachable())

		// nft.EndChain()

	}

	// Shared chain to dispatch packets based on cluster IPs
	nft.StartChain(&nftables.Chain{
		Name: "cluster-ips-dnat",
	})

	for _, svc := range services {
		key := svc.ObjectMeta.Namespace + "/" + svc.ObjectMeta.Name
		if svc.Spec.Type == "ExternalName" {
			continue
		}
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		rule.Reset()
		rule.IsIpDestAddress(svc.Spec.ClusterIP)
		rule.GoToChain("svc-" + string(svc.ObjectMeta.UID) + "-dnat")

		nft.AddRuleWithComment(rule, key)
	}

	nft.EndChain() // cluster-ips-dnat chain

	// Shared chain to dispatch packets based on cluster IPs
	nft.StartChain(&nftables.Chain{
		Name: "cluster-ips-filter",
	})

	for _, svc := range services {
		key := svc.ObjectMeta.Namespace + "/" + svc.ObjectMeta.Name
		if svc.Spec.Type == "ExternalName" {
			continue
		}
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		rule.Reset()
		rule.IsIpDestAddress(svc.Spec.ClusterIP)
		rule.Counter().RejectAsPortUnreachable()
		// rule.GoToChain("svc-" + string(svc.ObjectMeta.UID) + "-filter")

		nft.AddRuleWithComment(rule, key)
	}

	nft.EndChain() // cluster-ips-filter chain

	var podIpsServingClusterIpsList []string
	for k := range podIpsServingClusterIps {
		podIpsServingClusterIpsList = append(podIpsServingClusterIpsList, k)
	}
	sort.Strings(podIpsServingClusterIpsList)

	// Set up a masquerade for pods participating in a service
	nft.StartChain(&nftables.Chain{
		Name: "cluster-ip-backing-pods-masq",
	})
	for _, podIp := range podIpsServingClusterIpsList {
		rule.Reset()
		rule.IsIpSrcAddressStr(podIp)
		rule.IsIpDestAddress(podIp)
		rule.Counter()
		rule.Masquerade()
		nft.AddRuleWithComment(rule, "For "+podIpsServingClusterIps[podIp])
	}
	nft.EndChain()

	// Set up a fallback masquerade for non-pod traffic leaving to the cluster
	nft.StartChain(&nftables.Chain{
		Name: "cluster-outbound-masq",
	})
	nft.AddRuleWithComment(rule.IsIpSrcAddress(fc.NodeIP).Return(), "From our Node IP")
	for _, podNet := range fc.PodNets {
		nft.AddRuleWithComment(rule.IsIpSrcNetwork(podNet).Return(), "From our Pod CNI")
	}
	nft.AddRuleWithComment(rule.Counter().Masquerade(), "Foreign traffic from us to the cluster")
	nft.EndChain()

	///////////////////////////////////////////////////////////////
	// Static hook chains that send packets into our actual chains

	// TODO: only jump to cluster-ips chain if daddr has cluster-ip prefix
	// [ bitwise reg 1 = (reg=1 & 0x00f0ffff ) ^ 0x00000000 ]
	// https://godoc.org/github.com/google/nftables/expr#Bitwise

	nft.StartChain(&nftables.Chain{
		Name:     "hook-filter-forward",
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookForward,
		Priority: nftables.ChainPriorityFilter,
	})
	nft.AddRule(rule.JumpToChain("cluster-ips-filter")) // TODO: cluster IPs CIDR check
	nft.EndChain()

	nft.StartChain(&nftables.Chain{
		Name:     "hook-filter-local-out",
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
	})
	nft.AddRule(rule.JumpToChain("cluster-ips-filter")) // TODO: cluster IPs CIDR check
	nft.EndChain()

	nft.StartChain(&nftables.Chain{
		Name:     "hook-nat-fwd-in",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityNATDest,
	})
	nft.AddRule(rule.JumpToChain("cluster-ips-dnat")) // TODO: cluster IPs CIDR check
	nft.EndChain()

	nft.StartChain(&nftables.Chain{
		Name:     "hook-nat-local-out",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityMangle,
	})
	nft.AddRule(rule.JumpToChain("cluster-ips-dnat")) // TODO: cluster IPs CIDR check
	nft.EndChain()

	nft.StartChain(&nftables.Chain{
		Name:     "hook-nat-postrouting",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
	})
	nft.AddRule(rule.JumpToChain("cluster-ip-backing-pods-masq")) // TODO: local pod IP CIDR check & goto
	nft.AddRule(rule.OutIfaceName("lo").Return())                 // TODO: use interface index here
	nft.AddRule(rule.OutIfaceName(fc.VpnIface).GoToChain("cluster-outbound-masq"))
	// at this point we're leaving and not to the cluster, so maybe we're a pod looking for the internet
	for _, podNet := range fc.PodNets {
		nft.AddRuleWithComment(rule.IsIpSrcNetwork(podNet).Counter().Masquerade(), "From our Pod CNI")
	}
	nft.EndChain()

	nft.EndTable() // kube-pet table

	// TODO: handling multiple backends
	// TODO: actually use a vmap for this (verdict map) instead of two maps
	// dnat to numgen inc mod 2 map { 0 : 10.8.0.135 , 1 : 10.8.0.58 }
	// dnat to numgen inc mod 2 map { 0 : 10.8.0.135 , 1 : 10.8.0.58 } : numgen inc mod 2 map { 0 : 80 , 1 : 8080 }

	return nil
}
