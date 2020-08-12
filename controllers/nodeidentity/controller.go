package nodeidentity

import (
	"context"
	"log"
	"net"
	"runtime"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/tools/record"

	"github.com/danopia/kube-pet-node/podman"
)

func NewNodeIdentity(kubernetes *kubernetes.Clientset, nodeName string, conVersion *podman.DockerVersionReport, maxPods int, nodeIP net.IP, podNets []net.IPNet) (*node.NodeController, error) {

	podCIDRs := make([]string, len(podNets))
	for idx, podNet := range podNets {
		podCIDRs[idx] = (&podNet).String()
	}

	pNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"purpose":  "pet",
				"type":     "virtual-kubelet",
				"lifetime": "persistent",

				"kubernetes.io/role":     "pet",
				"kubernetes.io/hostname": nodeName,
				"kubernetes.io/arch":     runtime.GOARCH,
				"kubernetes.io/os":       runtime.GOOS,
			},
			Annotations: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			PodCIDRs: podCIDRs,
			// If ProviderID isn't a gce:// URI, the GKE autoscaler breaks in half as long as we're Ready
			ProviderID: "gce://kube-pet-node/bare-metal/" + nodeName,
			Taints: []corev1.Taint{{
				Key:    "kubernetes.io/pet-node",
				Value:  nodeName,
				Effect: "NoSchedule",
			}},
		},
	}

	if len(podCIDRs) > 0 {
		pNode.Spec.PodCIDR = podCIDRs[0]
	}

	nodeProvider, err := NewPetNodeProvider(pNode, conVersion, maxPods, nodeIP)
	if err != nil {
		return nil, err
	}

	return node.NewNodeController(nodeProvider, pNode,
		kubernetes.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1Beta1(kubernetes.CoordinationV1beta1().Leases(corev1.NamespaceNodeLease), nil),
		node.WithNodeStatusUpdateErrorHandler(func(ctx context.Context, err error) error {
			log.Println("NodeStatus Update err:", err)
			// if !k8serrors.IsNotFound(err) {
			// 	return err
			// }
			// log.G(ctx).Debug("node not found")
			// newNode := pNode.DeepCopy()
			// newNode.ResourceVersion = ""
			// _, err = client.CoreV1().Nodes().Create(newNode)
			// if err != nil {
			// 	return err
			// }
			// log.G(ctx).Debug("created new node")
			return nil
		}),
	)
}
