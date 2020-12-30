package selfprovision

import (
	"log"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"
	kubernetes "k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/danopia/kube-pet-node/podman"
)

// Controller !
type Controller struct {
	nodeName string
	vpnIface string
	cniNet   string

	coreV1Api    corev1client.CoreV1Interface
	nodeInformer kubeinformers.SharedInformerFactory
	podman       *podman.PodmanClient
}

// NewController !
func NewController(nodeName string, vpnIface string, cniNet string, kubernetes *kubernetes.Clientset, podman *podman.PodmanClient) *Controller {

	log.Println("SelfProvision: constructing controller")

	nodeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubernetes,
		5*time.Minute, // resync period
		// kubeinformers.WithNamespace(""),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", nodeName).String()
		}))

	return &Controller{
		nodeName:     nodeName,
		vpnIface:     vpnIface,
		cniNet:       cniNet,
		coreV1Api:    kubernetes.CoreV1(),
		nodeInformer: nodeInformerFactory,
		podman:       podman,
	}
}

