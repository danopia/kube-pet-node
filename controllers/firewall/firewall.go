package firewall

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/bep/debounce"
	"github.com/google/nftables"
	nftexpr "github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

type FirewallController struct {
	ServiceInformer   corev1informers.ServiceInformer
	EndpointsInformer corev1informers.EndpointsInformer
	Debounce          func(func())
}

func NewFirewallController(si corev1informers.ServiceInformer, ei corev1informers.EndpointsInformer) *FirewallController {
	return &FirewallController{
		ServiceInformer:   si,
		EndpointsInformer: ei,
		Debounce:          debounce.New(time.Second),
	}
}

func (fc *FirewallController) BuildConfig(nft *nftables.Conn, w io.Writer) error {

	endpoints, err := fc.EndpointsInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}
	services, err := fc.ServiceInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}

	// log.Println("Firewall:", len(endpoints), "endpoints")
	// log.Println("Firewall:", len(services), "services")

	table := &nftables.Table{
		Name:   "kube-pet",
		Family: nftables.TableFamilyIPv4,
	}

	nft.AddTable(table)
	w.Write([]byte("table ip kube-pet\n"))
	nft.DelTable(table)
	w.Write([]byte("delete table kube-pet\n\n"))

	nft.AddTable(table)
	w.Write([]byte("table ip kube-pet {\n"))

	endpointMap := make(map[string]*corev1.Endpoints, len(endpoints))
	for _, ep := range endpoints {
		key := ep.ObjectMeta.Namespace + "/" + ep.ObjectMeta.Name
		endpointMap[key] = ep
	}

	for _, svc := range services {
		key := svc.ObjectMeta.Namespace + "/" + svc.ObjectMeta.Name
		ep, ok := endpointMap[key]
		if !ok || svc.Spec.Type == "ExternalName" {
			continue
		}
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		svcChain := nft.AddChain(&nftables.Chain{
			Name:  "svc-" + string(svc.ObjectMeta.UID),
			Table: table,
		})
		w.Write([]byte("  chain " + svcChain.Name + " {\n"))

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

			var l4proto uint8
			switch port.Protocol {
			case "TCP":
				l4proto = unix.IPPROTO_TCP
			case "UDP":
				l4proto = unix.IPPROTO_UDP
			case "SCTP":
				l4proto = unix.IPPROTO_SCTP
			}

			portExprs := []nftexpr.Any{

				// tcp/udp/sctp
				// [ meta load l4proto => reg 1 ]
				&nftexpr.Meta{Key: nftexpr.MetaKeyL4PROTO, Register: 1},
				// [ cmp eq reg 1 0x00000006 ]
				&nftexpr.Cmp{
					Op:       nftexpr.CmpOpEq,
					Register: 1,
					Data:     []byte{l4proto},
				},

				// dport 80
				// [ payload load 2b @ transport header + 2 => reg 1 ]
				&nftexpr.Payload{
					DestRegister: 1,
					Base:         nftexpr.PayloadBaseTransportHeader,
					Offset:       2, // TODO
					Len:          2, // TODO
				},
				// [ cmp eq reg 1 0x00001600 ]
				&nftexpr.Cmp{
					Op:       nftexpr.CmpOpEq,
					Register: 1,
					Data:     parsePortForNft(uint16(port.Port)),
				},

				// counter
				// [ counter pkts 0 bytes 0 ]
				&nftexpr.Counter{},
			}
			w.Write([]byte(fmt.Sprintf("    %v dport %v counter ", port.Protocol, port.Port)))

			if len(tgtAddrs) == 0 {

				// reject with icmp type host-unreachable
				// [ reject type 0 code 1 ]
				portExprs = append(portExprs, &nftexpr.Reject{
					Type: 0,
					Code: 1, // host-unreachable
				})
				w.Write([]byte("reject with icmp type host-unreachable\n"))

			} else {

				// TODO: just uses first addr for now

				// dnat to 10.8.0.135 : 53
				// [ immediate reg 1 0x8700080a ]
				portExprs = append(portExprs, &nftexpr.Immediate{
					Register: 1,
					Data:     parseIpForNft(tgtAddrs[0]),
				})
				// [ immediate reg 2 0x00003500 ]
				portExprs = append(portExprs, &nftexpr.Immediate{
					Register: 2,
					Data:     parsePortForNft(tgtPorts[0]),
				})
				// [ nat dnat ip addr_min reg 1 addr_max reg 0 proto_min reg 2 proto_max reg 0 flags 0x2 ]
				portExprs = append(portExprs, &nftexpr.NAT{
					Type:        nftexpr.NATTypeDestNAT,
					Family:      unix.NFPROTO_IPV4,
					RegAddrMin:  1,
					RegAddrMax:  0,
					RegProtoMin: 2,
					RegProtoMax: 0,
					// Random      bool
					// FullyRandom bool
					// Persistent  bool
				})
				w.Write([]byte(fmt.Sprintf("dnat to %v : %v ", tgtAddrs[0], tgtPorts[0])))

			}

			nft.AddRule(&nftables.Rule{
				Table:    table,
				Chain:    svcChain,
				Exprs:    portExprs,
				UserData: MakeRuleComment(fmt.Sprintf("%v %v", key, port.Name)),
			})
			w.Write([]byte(fmt.Sprintf("comment \"%v\"\n", fmt.Sprintf("%v %v", key, port.Name))))

		}

		var healthyAddrs []string
		for _, subset := range ep.Subsets {
			for _, subAddr := range subset.Addresses {
				healthyAddrs = append(healthyAddrs, subAddr.IP)
			}
		}

		// TODO: reject if none healthy
		if len(healthyAddrs) > 0 {
			nft.AddRule(&nftables.Rule{
				Table: table,
				Chain: svcChain,
				Exprs: []nftexpr.Any{
					// [ payload load 1b @ network header + 9 => reg 1 ]
					&nftexpr.Payload{
						DestRegister: 1,
						Base:         nftexpr.PayloadBaseNetworkHeader,
						Offset:       9, // TODO
						Len:          1, // TODO
					},
					// [ cmp eq reg 1 0x00000001 ]
					&nftexpr.Cmp{
						Op:       nftexpr.CmpOpEq,
						Register: 1,
						Data:     []byte{unix.IPPROTO_ICMP},
					},
					//
					&nftexpr.Counter{},
					// [ immediate reg 1 0x08080808 ]
					&nftexpr.Immediate{
						Register: 1,
						Data:     parseIpForNft(healthyAddrs[0]),
					},
					// [ nat dnat ip addr_min reg 1 addr_max reg 0 ]
					&nftexpr.NAT{
						Type:       nftexpr.NATTypeDestNAT,
						Family:     unix.NFPROTO_IPV4,
						RegAddrMin: 1,
						RegAddrMax: 0,
					},
				},
			})
			w.Write([]byte("    ip protocol icmp counter dnat to " + healthyAddrs[0] + "\n"))
		}

		// reject with icmp type port-unreachable
		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: svcChain,
			Exprs: []nftexpr.Any{
				&nftexpr.Counter{},
				&nftexpr.Log{
					Key:  unix.NFTA_LOG_PREFIX,
					Data: []byte("uhh"),
				},
				&nftexpr.Reject{
					Type: 0,
					Code: 3, // port-unreachable
				},
			},
		})
		w.Write([]byte("    counter logreject with icmp type port-unreachable\n"))

		w.Write([]byte("  }\n\n"))
	}

	// chain cluster-ips {
	// 	type nat hook output priority 20;
	ipChain := nft.AddChain(&nftables.Chain{
		Name:     "cluster-ips",
		Table:    table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityNATDest,
	})
	w.Write([]byte("  chain " + ipChain.Name + " {\n"))
	w.Write([]byte("    type nat hook output priority -100;\n"))

	for _, svc := range services {
		key := svc.ObjectMeta.Namespace + "/" + svc.ObjectMeta.Name
		if svc.Spec.Type == "ExternalName" {
			continue
		}
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		nft.AddRule(&nftables.Rule{
			Table: table,
			Chain: ipChain,
			Exprs: []nftexpr.Any{
				// [ payload load 4b @ network header + 16 => reg 1 ]
				&nftexpr.Payload{
					DestRegister: 1,
					Base:         nftexpr.PayloadBaseNetworkHeader,
					Offset:       16, // TODO
					Len:          4,  // TODO
				},
				// [ cmp eq reg 1 0x4b0f060a ]
				&nftexpr.Cmp{
					Op:       nftexpr.CmpOpEq,
					Register: 1,
					Data:     parseIpForNft(svc.Spec.ClusterIP),
				},
				// [ immediate reg 0 jump -> service-b97565ce-95fa-464a-afd2-3fcfe3fe91fa ]
				&nftexpr.Verdict{
					Kind:  nftexpr.VerdictJump,
					Chain: "svc-" + string(svc.ObjectMeta.UID),
				},
			},
			UserData: MakeRuleComment(key),
		})
		w.Write([]byte("    ip daddr " + svc.Spec.ClusterIP + " jump svc-" + string(svc.ObjectMeta.UID) + "\n"))

	}

	w.Write([]byte("  }\n"))

	w.Write([]byte("}\n"))

	// #add rule cluster-ips routing ip daddr 10.6.0.10 counter dnat to numgen inc mod 2 map { 0 : 10.8.0.135 , 1 : 10.8.0.58 }

	return nil
}

func parseIpForNft(str string) []byte {
	ip := []byte(net.ParseIP(str).To4())
	// for i, j := 0, len(ip)-1; i < j; i, j = i+1, j-1 {
	//   ip[i], ip[j] = ip[j], ip[i]
	// }
	return ip
}
func parsePortForNft(port uint16) []byte {
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	return portBytes
}

const (
	// MaxCommentLength defines Maximum Length of Rule's Comment field
	MaxCommentLength = 127
)

// MakeRuleComment makes NFTNL_UDATA_RULE_COMMENT TLV. Length of TLV is 1 bytes
// as a result, the maximum comment length is 254 bytes.
func MakeRuleComment(s string) []byte {
	cl := len(s)
	c := s
	if cl > MaxCommentLength {
		cl = MaxCommentLength
		// Make sure that comment does not exceed maximum allowed length.
		c = s[:MaxCommentLength]
	}
	// Extra 3 bytes to carry Comment TLV type and length and taling 0x0
	comment := make([]byte, cl+3)
	comment[0] = 0x0
	comment[1] = uint8(cl + 1)
	copy(comment[2:], c)

	return comment
}

func (fc *FirewallController) Sync() {
	log.Println("Firewall: TODO: Sync()")

	nft := &nftables.Conn{}
	hasher := fnv.New32a()

	fc.BuildConfig(nft, hasher)
	hash := hasher.Sum(nil)
	log.Println("Firewall: config hash", hash)

	if err := nft.Flush(); err != nil {
		log.Println("nftables error:", err)
	}
	log.Println("Firewall: Sync() completed")
}

func (fc *FirewallController) Run(ctx context.Context) {
	log.Println("Firewall: Hello!")
	for !fc.EndpointsInformer.Informer().HasSynced() {
		log.Println("Firewall: Waiting for endpoints sync...")
		time.Sleep(2 * time.Second)
	}

	log.Println("Firewall: Setting up informer")
	// fc.EndpointsInformer.Informer().AddEventHandler(func(){})
	fc.EndpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(interface{}) {
			fc.Debounce(fc.Sync)
		},
		DeleteFunc: func(interface{}) {
			fc.Debounce(fc.Sync)
		},
		UpdateFunc: func(before, after interface{}) {
			// Ignore notificatins to control-plane election endpoints.
			//  "[T]here are plans for changing the leader election mechanism based on endpoints
			//   in favour of a similar approach based on config maps.
			//   This avoids continuously triggering “endpoint-changed” notifications
			//   to kube-proxy and kube-dns"
			if before, ok := before.(*corev1.Endpoints); ok {
				if before.ObjectMeta.Namespace == "kube-system" {
					if _, ok := before.ObjectMeta.Annotations["control-plane.alpha.kubernetes.io/leader"]; ok {
						return
					}
				}
				// log.Println("Updated:", before.ObjectMeta.Namespace, before.ObjectMeta.Name)
			}

			fc.Debounce(fc.Sync)
		},
	})

	// log.Println("Firewall: Seeding debounce")
	fc.Debounce(fc.Sync)
}
