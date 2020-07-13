package firewall

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	nftexpr "github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

type RuleBuilder struct {
	text  *strings.Builder
	exprs []nftexpr.Any
}

func NewRuleBuilder() *RuleBuilder {
	return &RuleBuilder{
		text: &strings.Builder{},
	}
}

func (rb *RuleBuilder) Exprs() []nftexpr.Any {
	return rb.exprs[:]
}
func (rb *RuleBuilder) String() string {
	return rb.text.String()[1:]
}

func (rb *RuleBuilder) Reset() {
	rb.text.Reset()
	rb.exprs = []nftexpr.Any{}
}

// "<tcp/udp/sctp> [...]"
// acts like a context setter
func (rb *RuleBuilder) IsL4Proto(id byte, name string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ meta load l4proto => reg 1 ]
		&nftexpr.Meta{Key: nftexpr.MetaKeyL4PROTO, Register: 1},
		// [ cmp eq reg 1 0x00000006 ]
		&nftexpr.Cmp{
			Op:       nftexpr.CmpOpEq,
			Register: 1,
			Data:     []byte{id},
		},
	)

	rb.text.WriteRune(' ')
	rb.text.WriteString(name)
	return rb
}

func (rb *RuleBuilder) IsTCP() *RuleBuilder {
	return rb.IsL4Proto(unix.IPPROTO_TCP, "tcp")
}
func (rb *RuleBuilder) IsUDP() *RuleBuilder {
	return rb.IsL4Proto(unix.IPPROTO_UDP, "udp")
}
func (rb *RuleBuilder) IsSCTP() *RuleBuilder {
	return rb.IsL4Proto(unix.IPPROTO_SCTP, "sctp")
}

// "[tcp/udp/sctp] dport <port>"
// seems to match for TCP, UDP, and SCTP at least
// so use any of those before using this
func (rb *RuleBuilder) IsDestPort(port uint16) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ payload load 2b @ transport header + 2 => reg 1 ]
		&nftexpr.Payload{
			DestRegister: 1,
			Base:         nftexpr.PayloadBaseTransportHeader,
			Offset:       2,
			Len:          2,
		},
		// [ cmp eq reg 1 0x00001600 ]
		&nftexpr.Cmp{
			Op:       nftexpr.CmpOpEq,
			Register: 1,
			Data:     parsePortForNft(port),
		},
	)

	rb.text.WriteString(" dport ")
	rb.text.WriteString(strconv.FormatUint(uint64(port), 10))
	return rb
}

func (rb *RuleBuilder) Counter() *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ counter pkts 0 bytes 0 ]
		&nftexpr.Counter{},
	)

	rb.text.WriteString(" counter")
	return rb
}

func (rb *RuleBuilder) RejectAsHostUnreachable() *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ reject type 0 code 1 ]
		&nftexpr.Reject{
			Type: 0,
			Code: 1, // host-unreachable
		},
	)

	rb.text.WriteString(" reject with icmp type host-unreachable")
	return rb
}

func (rb *RuleBuilder) RejectAsPortUnreachable() *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ reject type 0 code 1 ]
		&nftexpr.Reject{
			Type: 0,
			Code: 3, // port-unreachable
		},
	)

	rb.text.WriteString(" reject with icmp type port-unreachable")
	return rb
}

func (rb *RuleBuilder) Accept() *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ immediate reg 0 accept ]
		&nftexpr.Verdict{
			Kind: nftexpr.VerdictAccept,
		},
	)

	rb.text.WriteString(" accept")
	return rb
}

func (rb *RuleBuilder) TranslateIPv4Address(ip string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ immediate reg 1 0x8700080a ]
		&nftexpr.Immediate{
			Register: 1,
			Data:     parseIpForNft(ip),
		},
		// [ nat dnat ip addr_min reg 1 addr_max reg 0flags 0x2 ]
		&nftexpr.NAT{
			Type:       nftexpr.NATTypeDestNAT,
			Family:     unix.NFPROTO_IPV4,
			RegAddrMin: 1,
			RegAddrMax: 0,
		},
	)

	rb.text.WriteString(" dnat to ")
	rb.text.WriteString(ip)
	return rb
}

func (rb *RuleBuilder) TranslateIPv4Destination(ip string, port uint16) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ immediate reg 1 0x8700080a ]
		&nftexpr.Immediate{
			Register: 1,
			Data:     parseIpForNft(ip),
		},
		// [ immediate reg 2 0x00003500 ]
		&nftexpr.Immediate{
			Register: 2,
			Data:     parsePortForNft(port),
		},
		// [ nat dnat ip addr_min reg 1 addr_max reg 0 proto_min reg 2 proto_max reg 0 flags 0x2 ]
		&nftexpr.NAT{
			Type:        nftexpr.NATTypeDestNAT,
			Family:      unix.NFPROTO_IPV4,
			RegAddrMin:  1,
			RegAddrMax:  0,
			RegProtoMin: 2,
			RegProtoMax: 0,
			// Random      bool
			// FullyRandom bool
			// Persistent  bool
		},
	)

	rb.text.WriteString(" dnat to ")
	rb.text.WriteString(ip)
	rb.text.WriteRune(':')
	rb.text.WriteString(strconv.FormatUint(uint64(port), 10))
	return rb
}

func (rb *RuleBuilder) IsIpProtocol(id byte, name string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
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
			Data:     []byte{id},
		},
	)

	rb.text.WriteString(" ip protocol ")
	rb.text.WriteString(name)
	return rb
}

func (rb *RuleBuilder) IsICMP() *RuleBuilder {
	return rb.IsIpProtocol(unix.IPPROTO_ICMP, "icmp")
}

func (rb *RuleBuilder) IsIpDestAddress(ip string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ payload load 4b @ network header + 16 => reg 1 ]
		&nftexpr.Payload{
			DestRegister: 1,
			Base:         nftexpr.PayloadBaseNetworkHeader,
			Offset:       16,
			Len:          4,
		},
		// [ cmp eq reg 1 0x4b0f060a ]
		&nftexpr.Cmp{
			Op:       nftexpr.CmpOpEq,
			Register: 1,
			Data:     parseIpForNft(ip),
		},
	)

	rb.text.WriteString(" ip daddr ")
	rb.text.WriteString(ip)
	return rb
}

func (rb *RuleBuilder) IsIpSrcAddress(ip string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ payload load 4b @ network header + 16 => reg 1 ]
		&nftexpr.Payload{
			DestRegister: 1,
			Base:         nftexpr.PayloadBaseNetworkHeader,
			Offset:       12,
			Len:          4,
		},
		// [ cmp eq reg 1 0x4b0f060a ]
		&nftexpr.Cmp{
			Op:       nftexpr.CmpOpEq,
			Register: 1,
			Data:     parseIpForNft(ip),
		},
	)

	rb.text.WriteString(" ip saddr ")
	rb.text.WriteString(ip)
	return rb
}

// call into another chain and jump back afterwards if it didn't have a verdict
func (rb *RuleBuilder) JumpToChain(chain string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ immediate reg 0 jump -> other-chain ]
		&nftexpr.Verdict{
			Kind:  nftexpr.VerdictJump,
			Chain: chain,
		},
	)

	rb.text.WriteString(" jump ")
	rb.text.WriteString(chain)
	return rb
}

// switch to another chain and don't come back afterwards
// (still respect returning to any previous jumps)
func (rb *RuleBuilder) GoToChain(chain string) *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ immediate reg 0 goto -> other-chain ]
		&nftexpr.Verdict{
			Kind:  nftexpr.VerdictGoto,
			Chain: chain,
		},
	)

	rb.text.WriteString(" goto ")
	rb.text.WriteString(chain)
	return rb
}

// perform automatic source NAT
func (rb *RuleBuilder) Masquerade() *RuleBuilder {
	rb.exprs = append(rb.exprs,
		// [ masq ]
		&nftexpr.Masq{},
	)

	rb.text.WriteString(" masquerade")
	return rb
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
