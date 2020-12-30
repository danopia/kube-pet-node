package controller

import (
	"context"
	"log"
	"net"
	"time"

	// "k8s.io/client-go/tools/clientcmd"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	// coordv1 "k8s.io/api/coordination/v1beta1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	// corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	// "github.com/virtual-kubelet/virtual-kubelet/log"

	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/danopia/kube-pet-node/controllers/autoupgrade"
	"github.com/danopia/kube-pet-node/controllers/caching"
	"github.com/danopia/kube-pet-node/controllers/firewall"
	"github.com/danopia/kube-pet-node/controllers/kubeapi"
	"github.com/danopia/kube-pet-node/controllers/nodeidentity"
	"github.com/danopia/kube-pet-node/controllers/pods"
	"github.com/danopia/kube-pet-node/controllers/volumes"
	// "github.com/danopia/kube-pet-node/podman"
)

type PetNode struct {
	// our clients
	NodeName   string
	NodeUID    types.UID
	PodManager *pods.PodManager
	Kubernetes *kubernetes.Clientset

	// our controllers
	Firewall *firewall.FirewallController
	// Pods          *pods.FirewallController
	// Volumes *volumes.VolumesController

	// virtual-kubelet controllers
	NodeRunner *node.NodeController
	PodRunner  *node.PodController

	// kubernetes object caches
	// PodInformer       corev1informers.PodInformer
	// SecretInformer    corev1informers.SecretInformer
	// ConfigMapInformer corev1informers.ConfigMapInformer
	// ServiceInformer   corev1informers.ServiceInformer
}

func NewPetNode(ctx context.Context, nodeName string, podManager *pods.PodManager, kubernetes *kubernetes.Clientset, maxPods int, vpnIface string, nodeIP net.IP, podNets []net.IPNet, cniNet string) (*PetNode, error) {

	autoUpgrade, err := autoupgrade.NewAutoUpgrade()
	if err != nil {
		return nil, err
	}

	petVersion := "development"
	if autoUpgrade.SelfVersion != nil {
		petVersion = autoUpgrade.SelfVersion.String()
	}

	conVersion, err := podManager.RuntimeVersionReport(ctx)
	if err != nil {
		return nil, err
	}

	nodeRunner, err := nodeidentity.NewNodeIdentity(ctx, kubernetes, nodeName, petVersion, conVersion, maxPods, nodeIP, podNets)
	if err != nil {
		return nil, err
	}

	eb := record.NewBroadcaster()
	eb.StartLogging(func(a string, b ...interface{}) {
		log.Printf("K8S Event: "+a, b...)
	})
	// eb.StartLogging(log.G(context.TODO()).Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubernetes.CoreV1().Events("")})

	kubeletEvents := eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "kubelet", Host: nodeName})
	// Using ObjectReference for events as the node maybe not cached; refer to #42701 for detail.
	nodeRef := &corev1.ObjectReference{
		Kind:      "Node",
		Name:      nodeName,
		UID:       types.UID(nodeName),
		Namespace: "",
	}

	// setup other things... if we have an IP
	var podInformerFactory kubeinformers.SharedInformerFactory
	var scmInformerFactory kubeinformers.SharedInformerFactory
	var podRunner *node.PodController
	var firewallRunner *firewall.FirewallController
	var kubeApiRunner *kubeapi.KubeApi

	if len(nodeIP) > 0 {
		caching := caching.NewController(kubernetes)

		volumes := volumes.NewVolumesController(kubernetes, caching, podManager.GetPodman())

		//https://github.com/virtual-kubelet/virtual-kubelet/blob/3ec3b14e49d0c2f335ca049155d1ee94b2baf35f/cmd/virtual-kubelet/internal/commands/root/root.go

		// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
		podInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(
			kubernetes,
			5*time.Minute, // resync period
			// kubeinformers.WithNamespace(""),
			kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
			}))
		podInformer := podInformerFactory.Core().V1().Pods()

		// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
		scmInformerFactory = kubeinformers.NewSharedInformerFactoryWithOptions(kubernetes, 15*time.Minute)
		// Create a secret informer and a config map informer so we can pass their listers to the resource manager.
		secretInformer := scmInformerFactory.Core().V1().Secrets()
		configMapInformer := scmInformerFactory.Core().V1().ConfigMaps()
		serviceInformer := scmInformerFactory.Core().V1().Services()
		endpointsInformer := scmInformerFactory.Core().V1().Endpoints()

		podRunner, err = node.NewPodController(node.PodControllerConfig{
			PodClient: kubernetes.CoreV1(),
			Provider:  pods.NewPodmanProvider(podManager, caching, volumes, kubeletEvents, cniNet),

			PodInformer:       podInformer,
			EventRecorder:     kubeletEvents,
			SecretInformer:    secretInformer,
			ConfigMapInformer: configMapInformer,
			ServiceInformer:   serviceInformer,
		})
		if err != nil {
			return nil, err
		}

		firewallRunner = firewall.NewFirewallController(nodeName, vpnIface, nodeIP, podNets, serviceInformer, endpointsInformer)
		go firewallRunner.Run(ctx)

		kubeApiRunner, err = kubeapi.NewKubeApi(kubernetes, podManager, nodeName, nodeIP)
		if err != nil {
			return nil, err
		}
		go kubeApiRunner.Run(ctx)

		go autoUpgrade.Run(ctx, configMapInformer)
	}

	petNode := &PetNode{
		NodeName:   nodeName,
		NodeUID:    types.UID(nodeName), // to be replaced
		Kubernetes: kubernetes,
		PodManager: podManager,

		NodeRunner: nodeRunner,
		PodRunner:  podRunner,
		Firewall:   firewallRunner,

		// PodInformer:       podInformer,
		// SecretInformer:    secretInformer,
		// ConfigMapInformer: configMapInformer,
		// ServiceInformer:   serviceInformer,
	}

	go nodeRunner.Run(ctx)
	if len(nodeIP) > 0 {
		go podRunner.Run(ctx, 1) // number of sync workers
	}

	log.Println("Waiting for node runner...")
	<-nodeRunner.Ready()
	log.Println("Node runner ready")

	if node, err := kubernetes.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{}); err == nil {
		petNode.NodeUID = node.UID
		nodeRef.UID = node.UID
	} else {
		log.Println("BUG: We didn't see our own node after starting up!")
		return nil, err
	}

	if len(nodeIP) == 0 {
		kubeletEvents.Eventf(nodeRef, corev1.EventTypeWarning, "Configuring", "TODO: Configuring kube-pet networking.")
		log.Println("TODO!!!")

	} else {

		log.Println("Ensuring static pods...")
		if err := petNode.EnsureStaticPods(ctx); err != nil {
			log.Println("WARN: Static pods couldn't be ensured because", err)
		}

		podInformerFactory.Start(ctx.Done())
		scmInformerFactory.Start(ctx.Done())
		log.Println("Informers started")

		kubeletEvents.Eventf(nodeRef, corev1.EventTypeNormal, "Starting" /*StartingKubelet*/, "Starting kube-pet-node.")
	}

	return petNode, nil
}
