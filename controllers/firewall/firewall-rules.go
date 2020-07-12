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

	for _, svc := range services {
		key := svc.ObjectMeta.Namespace + "/" + svc.ObjectMeta.Name
		ep, ok := endpointMap[key]
		if !ok || svc.Spec.Type == "ExternalName" {
			continue
		}
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		nft.StartBasicChain("svc-" + string(svc.ObjectMeta.UID))

		for _, port := range svc.Spec.Ports {

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

			rule.Counter()

			if len(tgtAddrs) == 0 {
				rule.RejectAsHostUnreachable()
			} else {
				// TODO: just uses first addr for now
				rule.TranslateIPv4Destination(tgtAddrs[0], tgtPorts[0])
			}

			portName := port.Name
			if portName == "" {
				portName = "default"
			}
			nft.AddRuleWithComment(rule, fmt.Sprintf("%v %v %v", key, port.Protocol, portName))
		}

		// Route ICMP traffic to anything that's at all healthy

		var healthyAddrs []string
		for _, subset := range ep.Subsets {
			for _, subAddr := range subset.Addresses {
				healthyAddrs = append(healthyAddrs, subAddr.IP)
			}
		}

		rule.Reset()
		rule.IsICMP()
		rule.Counter()

		if len(healthyAddrs) == 0 {
			rule.RejectAsHostUnreachable()
		} else {
			// TODO: just uses first addr for now
			rule.TranslateIPv4Address(healthyAddrs[0])
		}

		nft.AddRuleWithComment(rule, key+" ICMP")

		// Reject all other (unknown port) traffic
		nft.AddRule(rule.Counter().RejectAsPortUnreachable())

		nft.EndChain()
	}

	// Shared chain to dispatch packets based on cluster IPs
	nft.StartChain(&nftables.Chain{
		Name:     "cluster-ips",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityNATDest,
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
		rule.JumpToChain("svc-" + string(svc.ObjectMeta.UID))

		nft.AddRuleWithComment(rule, key)
	}

	nft.EndChain() // cluster-ips chain

	// Shared chain to dispatch packets based on cluster IPs
	// Forward so that we catch
	nft.StartChain(&nftables.Chain{
		Name:     "cluster-ips-fwd",
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookForward,
		Priority: nftables.ChainPriorityNATDest,
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
		rule.JumpToChain("svc-" + string(svc.ObjectMeta.UID))

		nft.AddRuleWithComment(rule, key)
	}

	nft.EndChain() // cluster-ips chain

	nft.EndTable() // kube-pet table

	// TODO: handling multiple backends
	// dnat to numgen inc mod 2 map { 0 : 10.8.0.135 , 1 : 10.8.0.58 }
	// dnat to numgen inc mod 2 map { 0 : 10.8.0.135 , 1 : 10.8.0.58 } : numgen inc mod 2 map { 0 : 80 , 1 : 8080 }

	return nil
}
