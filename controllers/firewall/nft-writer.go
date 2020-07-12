package firewall

import (
	"fmt"
	"io"
	"strconv"

	"github.com/google/nftables"
	"golang.org/x/sys/unix"
)

type NftWriter struct {
	conn *nftables.Conn
	text io.Writer

	table *nftables.Table
	chain *nftables.Chain
}

func NewNftWriter(nft *nftables.Conn, w io.Writer) *NftWriter {
	return &NftWriter{
		conn: nft,
		text: w,
	}
}

func (nft *NftWriter) StartTableReplacement(name string, family nftables.TableFamily) *nftables.Table {
	nft.EndTable()

	nft.table = &nftables.Table{
		Name:   name,
		Family: family,
	}
	familyStr := "ip" // TODO!!! IPv4 only

	nft.conn.AddTable(nft.table)
	nft.text.Write([]byte("table " + familyStr + " " + name + "\n"))
	nft.conn.DelTable(nft.table)
	nft.text.Write([]byte("delete table " + name + "\n"))

	nft.conn.AddTable(nft.table)
	nft.text.Write([]byte("\ntable " + familyStr + " " + name + " {"))

	return nft.table
}

func (nft *NftWriter) EndTable() {
	if nft.table != nil {
		nft.EndChain()

		nft.text.Write([]byte("}\n"))
		nft.table = nil
	}
}

func (nft *NftWriter) StartBasicChain(name string) *nftables.Chain {
	return nft.StartChain(&nftables.Chain{Name: name})
}

func (nft *NftWriter) StartChain(chain *nftables.Chain) *nftables.Chain {
	if nft.table == nil {
		return nil
	}
	nft.EndChain()

	chain.Table = nft.table
	nft.chain = chain

	nft.conn.AddChain(nft.chain)
	nft.text.Write([]byte("\n  chain " + nft.chain.Name + " {\n"))

	if chain.Type != "" {
		nft.text.Write([]byte("    type " + chain.Type))
		switch chain.Hooknum {
		case unix.NF_INET_PRE_ROUTING:
			nft.text.Write([]byte(" hook prerouting"))
		case unix.NF_INET_LOCAL_IN:
			nft.text.Write([]byte(" hook input"))
		case unix.NF_INET_FORWARD:
			nft.text.Write([]byte(" hook forward"))
		case unix.NF_INET_LOCAL_OUT:
			nft.text.Write([]byte(" hook output"))
		case unix.NF_INET_POST_ROUTING:
			nft.text.Write([]byte(" hook postrouting"))
		default:
			nft.text.Write([]byte(" hook TODO!"))
		}
		nft.text.Write([]byte(" priority " + strconv.Itoa(int(chain.Priority))))
		nft.text.Write([]byte(";\n"))
	}

	return nft.chain
}

func (nft *NftWriter) EndChain() {
	if nft.chain != nil {
		nft.text.Write([]byte("  }\n"))
		nft.chain = nil
	}
}

func (nft *NftWriter) AddRule(rule *RuleBuilder) {
	nft.conn.AddRule(&nftables.Rule{
		Table: nft.table,
		Chain: nft.chain,
		Exprs: rule.Exprs(),
	})
	nft.text.Write([]byte(fmt.Sprintf("    %v\n", rule.String())))
	rule.Reset()
}

func (nft *NftWriter) AddRuleWithComment(rule *RuleBuilder, comment string) {
	nft.conn.AddRule(&nftables.Rule{
		Table:    nft.table,
		Chain:    nft.chain,
		Exprs:    rule.Exprs(),
		UserData: makeRuleComment(comment),
	})
	nft.text.Write([]byte(fmt.Sprintf("    %v comment \"%v\"\n", rule.String(), comment)))
	rule.Reset()
}

const (
	// maxCommentLength defines Maximum Length of Rule's Comment field
	maxCommentLength = 127
)

// makeRuleComment makes NFTNL_UDATA_RULE_COMMENT TLV. Length of TLV is 1 bytes
// as a result, the maximum comment length is 254 bytes.
func makeRuleComment(s string) []byte {
	cl := len(s)
	c := s
	if cl > maxCommentLength {
		cl = maxCommentLength
		// Make sure that comment does not exceed maximum allowed length.
		c = s[:maxCommentLength]
	}
	// Extra 3 bytes to carry Comment TLV type and length and taling 0x0
	comment := make([]byte, cl+3)
	comment[0] = 0x0
	comment[1] = uint8(cl + 1)
	copy(comment[2:], c)

	return comment
}
