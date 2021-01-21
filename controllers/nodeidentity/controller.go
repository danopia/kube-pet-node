package nodeidentity

import (
	"context"
	"log"
	"net"
	"runtime"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	// "k8s.io/client-go/tools/record"

	"github.com/danopia/kube-pet-node/pkg/podman"
)

func NewNodeIdentity(ctx context.Context, kubernetes *kubernetes.Clientset, nodeName string, petVersion string, conVersion *podman.DockerVersionReport, maxPods int, nodeIP net.IP, podNets []net.IPNet) (*node.NodeController, error) {

	podCIDRs := make([]string, len(podNets))
	for idx, podNet := range podNets {
		podCIDRs[idx] = (&podNet).String()
	}

	pNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nodeName,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: corev1.NodeSpec{
			PodCIDRs: podCIDRs,
			// If ProviderID isn't a gce:// URI, the GKE autoscaler breaks in half as long as we're Ready
			ProviderID: "gce://kube-pet-node/kube-pet-node/" + nodeName,
			Taints: []corev1.Taint{{
				Key:    "kubernetes.io/pet-node", // TODO: is nonstandard
				Value:  nodeName,
				Effect: "NoSchedule",
			}},
		},
	}

	// If our node already exists, use it for inspiration
	existingNode, err := kubernetes.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err == nil {
		existingNode.ObjectMeta.DeepCopyInto(&pNode.ObjectMeta)
	} else if !errors.IsNotFound(err) {
		return nil, err
	}

	pNode.ObjectMeta.Labels["purpose"] = "pet"
	pNode.ObjectMeta.Labels["type"] = "virtual-kubelet"
	pNode.ObjectMeta.Labels["lifetime"] = "persistent"

	pNode.ObjectMeta.Labels["kubernetes.io/role"] = "pet"
	pNode.ObjectMeta.Labels["kubernetes.io/hostname"] = strings.TrimPrefix(nodeName, "pet-")
	pNode.ObjectMeta.Labels["kubernetes.io/arch"] = runtime.GOARCH
	pNode.ObjectMeta.Labels["kubernetes.io/os"] = runtime.GOOS

	// i made this one up to deal with external-dns
	pNode.ObjectMeta.Annotations["kubernetes.io/node.class"] = "kube-pet"

	if len(podCIDRs) > 0 {
		pNode.Spec.PodCIDR = podCIDRs[0]
	}

	nodeProvider, err := NewPetNodeProvider(pNode, petVersion, conVersion, maxPods, nodeIP)
	if err != nil {
		return nil, err
	}

	return node.NewNodeController(nodeProvider, pNode,
		kubernetes.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1(kubernetes.CoordinationV1().Leases(corev1.NamespaceNodeLease), 0),
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
