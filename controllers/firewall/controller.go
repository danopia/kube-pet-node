package firewall

import (
	"bytes"
	"context"
	"hash"
	"hash/fnv"
	"log"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/bep/debounce"
	"github.com/google/nftables"
)

type FirewallController struct {
	ServiceInformer   corev1informers.ServiceInformer
	EndpointsInformer corev1informers.EndpointsInformer
	Debounce          func(func())
	LatestConfigHash  []byte
}

func NewFirewallController(si corev1informers.ServiceInformer, ei corev1informers.EndpointsInformer) *FirewallController {
	return &FirewallController{
		ServiceInformer:   si,
		EndpointsInformer: ei,
		Debounce:          debounce.New(time.Second),
	}
}

type HashingBuilder struct {
	builder *strings.Builder
	hasher  hash.Hash32
}

func (hb *HashingBuilder) Write(p []byte) (int, error) {
	hb.builder.Write(p)
	return hb.hasher.Write(p)
}

// This is run by the debouncer so we don't have any error checking above us
func (fc *FirewallController) Sync() {
	// log.Println("Firewall: Starting Sync()")

	nft := &nftables.Conn{}
	writer := &HashingBuilder{
		builder: &strings.Builder{},
		hasher:  fnv.New32a(),
	}
	// hasher := fnv.New32a()

	nftWriter := NewNftWriter(nft, writer)
	fc.BuildConfig(nftWriter)

	hash := writer.hasher.Sum(nil)
	// log.Println("Firewall: config hash", hash)

	if changed := bytes.Compare(fc.LatestConfigHash, hash) != 0; changed {
		// log.Println("Firewall: New config:\n", writer.builder.String())
		log.Println("Firewall: Configuration changed, new hash:", hash)

		if err := nft.Flush(); err != nil {
			log.Println("Firewall: nftables error:", err)
		} else {
			log.Println("Firewall: Kernel table updated :)")
			fc.LatestConfigHash = hash
		}
	} else {
		log.Println("Firewall: Up to date")
	}
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
