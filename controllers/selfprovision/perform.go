package selfprovision

import (
	// "bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	// "hash"
	// "hash/fnv"
	// "net"
	// "strings"
	// "time"

	"github.com/danopia/kube-pet-node/pkg/wireguard"
)

// The annotations that we care about on a node
const clusterCfgAnnotation = "pet.wg69.net/cluster-configmap"
const nodeCfgAnnotation = "pet.wg69.net/node-configmap"
const wgPubKeyAnnotation = "pet.wg69.net/wg-pubkey"

// Perform !
func (ctl *Controller) Perform(ctx context.Context) error {
	log.Println("SelfProvision: Starting Perform() sequence!")

	doneCtx, doneFunc := context.WithCancel(ctx)

	clusterConfigMapKey := ""
	nodeConfigMapKey := ""

	wgIface := wireguard.ByName(ctl.vpnIface)
	wgPubKey := ""

	var clusterCfg *ClusterConfig

	processClusterConfigMap := func(val string) error {
		clusterConfigMapKey = val
		log.Println("received cluster config map key", val)

		parts := strings.Split(val, "/")
		configMap, err := ctl.coreV1Api.ConfigMaps(parts[0]).Get(doneCtx, parts[1], metav1.GetOptions{})
		if err != nil {
			return err
		}

		if clusterData, ok := configMap.Data["Cluster"]; ok {
			clusterCfg, err = ParseClusterCfg(clusterData)
			if err != nil {
				return err
			}

			log.Println("Received cluster pet configuration:", clusterCfg)

		} else {
			return fmt.Errorf("No Cluster key found on %v", val)
		}

		return nil
	}

	processNodeConfigMap := func(val string) error {
		nodeConfigMapKey = val
		log.Println("received node config map key", val)

		parts := strings.Split(val, "/")
		configMap, err := ctl.coreV1Api.ConfigMaps(parts[0]).Get(doneCtx, parts[1], metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Step 6. Write out CNI file
		// sudo /usr/bin/tee /etc/cni/net.d/*.conflist

		// Step 7. Write new Wireguard configuration to /etc/wireguard
		// Step 7b. Generate new private key if none yet

		// Step 8. Update cluster state with our keypair information
		// Step 8b. Central router reconfigures to add us as a peer
		// Step 8c. Wait, idk, 5 seconds? for router to update

		// Step 9. Enable/Bring up WireGuard bridge with systemctl
		// sudo /bin/systemctl enable wg-quick@*
		// sudo /bin/systemctl start wg-quick@*

		// Step 10. Confirm that iface is seen with correct IP (not a lot to do if the IP is wrong though)
		// sudo /usr/bin/wg *

		// Step 11. Exit without error, let the process get restarted normally.

		if false {
			doneFunc()
		}

		log.Println("Node config:", configMap)
		return nil
	}

	observeNode := func(node *corev1.Node) error {

		// Step 1) Receive the cluster-wide configuration from the pet-controller
		if val, ok := node.ObjectMeta.Annotations[clusterCfgAnnotation]; ok {
			if val != clusterConfigMapKey {
				err := processClusterConfigMap(val)
				if err != nil {
					return err
				}
			}
		}

		// Step 2) If SelfProvision is set, submit our WireGuard public key,
		//         creating & storing a new keypair on disk if necesary
		if clusterCfg != nil && clusterCfg.WireguardMode == "SelfProvision" {
			if wgPubKey == "" {
				pubKey, err := wgIface.EnsureKeyMaterial()
				if err != nil {
					return err
				}
				log.Println("Our Wireguard public key is", pubKey)
				wgPubKey = pubKey
			}

			annotationPatch, err := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]string{
						wgPubKeyAnnotation: wgPubKey,
					},
				},
			})
			if err != nil {
				return err
			}

			if val, ok := node.ObjectMeta.Annotations[wgPubKeyAnnotation]; ok {
				if val != wgPubKey {
					if _, err := ctl.coreV1Api.Nodes().Patch(doneCtx, node.ObjectMeta.Name,
						types.MergePatchType, annotationPatch, metav1.PatchOptions{}); err != nil {
						return err
					}
				}
			} else {
				if _, err := ctl.coreV1Api.Nodes().Patch(doneCtx, node.ObjectMeta.Name,
					types.MergePatchType, annotationPatch, metav1.PatchOptions{}); err != nil {
					return err
				}
			}

		}

		// Step 3) Receive and apply our node-specific configuration, then shut down
		if val, ok := node.ObjectMeta.Annotations[nodeCfgAnnotation]; ok {
			if val != nodeConfigMapKey {
				err := processNodeConfigMap(val)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	ctl.nodeInformer.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := obj.(*corev1.Node)
			if err := observeNode(node); err != nil {
				panic(err)
			}

		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			node := newObj.(*corev1.Node)
			if err := observeNode(node); err != nil {
				panic(err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			panic("SelfProvision: our node disappeared while we were running")
		},
	})

	ctl.nodeInformer.Start(doneCtx.Done())

	log.Println("SelfProvision: Waiting for completion...")
	<-doneCtx.Done()

	return nil
}
