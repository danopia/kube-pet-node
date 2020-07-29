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
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	// "github.com/virtual-kubelet/virtual-kubelet/log"

	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/danopia/kube-pet-node/controllers/firewall"
	"github.com/danopia/kube-pet-node/controllers/kubeapi"
	"github.com/danopia/kube-pet-node/controllers/autoupgrade"
	"github.com/danopia/kube-pet-node/controllers/pods"
	// "github.com/danopia/kube-pet-node/podman"
)

// type NP struct {}
// func (np *NP) NotifyNodeStatus(	)

type PetNode struct {
	// our clients
	NodeName   string
	PodManager *pods.PodManager
	Kubernetes *kubernetes.Clientset

	// our controllers
	Firewall *firewall.FirewallController
	// Pods          *pods.FirewallController

	// virtual-kubelet controllers
	NodeRunner *node.NodeController
	PodRunner  *node.PodController

	// kubernetes object caches
	PodInformer       corev1informers.PodInformer
	SecretInformer    corev1informers.SecretInformer
	ConfigMapInformer corev1informers.ConfigMapInformer
	ServiceInformer   corev1informers.ServiceInformer
}

func NewPetNode(ctx context.Context, nodeName string, podManager *pods.PodManager, kubernetes *kubernetes.Clientset, maxPods int, nodeIP net.IP, podNets []net.IPNet, cniNet string) (*PetNode, error) {

	podCIDRs := make([]string, len(podNets))
	for idx, podNet := range podNets {
		podCIDRs[idx] = (&podNet).String()
	}

	pNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"purpose":                "pet",
				"type":                   "virtual-kubelet",
				"kubernetes.io/role":     "pet",
				"kubernetes.io/hostname": nodeName,
			},
		},
		Spec: corev1.NodeSpec{
			PodCIDRs:   podCIDRs,
			ProviderID: "pet://" + nodeName,
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

	conVersion, err := podManager.RuntimeVersionReport(ctx)
	if err != nil {
		return nil, err
	}

	nodeProvider, err := NewPetNodeProvider(pNode, conVersion, maxPods, nodeIP)
	if err != nil {
		return nil, err
	}

	nodeRunner, err := node.NewNodeController(nodeProvider, pNode,
		kubernetes.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1Beta1(kubernetes.CoordinationV1beta1().Leases(corev1.NamespaceNodeLease), nil),
		// node.WithNodeStatusUpdateErrorHandler(func(ctx context.Context, err error) error {
		// 	if !k8serrors.IsNotFound(err) {
		// 		return err
		// 	}
		// 	log.G(ctx).Debug("node not found")
		// 	newNode := pNode.DeepCopy()
		// 	newNode.ResourceVersion = ""
		// 	_, err = client.CoreV1().Nodes().Create(newNode)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	log.G(ctx).Debug("created new node")
		// 	return nil
		// }),
	)
	if err != nil {
		return nil, err
	}

	//https://github.com/virtual-kubelet/virtual-kubelet/blob/3ec3b14e49d0c2f335ca049155d1ee94b2baf35f/cmd/virtual-kubelet/internal/commands/root/root.go

	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		kubernetes,
		1*time.Minute, // resync period
		// kubeinformers.WithNamespace(""),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", pNode.Name).String()
		}))
	podInformer := podInformerFactory.Core().V1().Pods()

	// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
	scmInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubernetes, 1*time.Minute)
	// Create a secret informer and a config map informer so we can pass their listers to the resource manager.
	secretInformer := scmInformerFactory.Core().V1().Secrets()
	configMapInformer := scmInformerFactory.Core().V1().ConfigMaps()
	serviceInformer := scmInformerFactory.Core().V1().Services()
	endpointsInformer := scmInformerFactory.Core().V1().Endpoints()

	// rm, err := manager.NewResourceManager(podInformer.Lister(), secretInformer.Lister(), configMapInformer.Lister(), serviceInformer.Lister())
	// if err != nil {
	// 	return errors.Wrap(err, "could not create resource manager")
	// }

	eb := record.NewBroadcaster()
	eb.StartLogging(func(a string, b ...interface{}) {
		log.Printf("K8S Event: "+a, b...)
	})
	// eb.StartLogging(log.G(context.TODO()).Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubernetes.CoreV1().Events("default")})

	kubeletEvents := eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "kubelet", Host: pNode.Name})
	// Using ObjectReference for events as the node maybe not cached; refer to #42701 for detail.
	nodeRef := &corev1.ObjectReference{
		Kind:      "Node",
		Name:      pNode.Name,
		UID:       types.UID(pNode.Name),
		Namespace: "",
	}

	// setup other things
	podRunner, err := node.NewPodController(node.PodControllerConfig{
		PodClient: kubernetes.CoreV1(),
		Provider:  pods.NewPodmanProvider(podManager, cniNet),

		PodInformer:       podInformer,
		EventRecorder:     kubeletEvents,
		SecretInformer:    secretInformer,
		ConfigMapInformer: configMapInformer,
		ServiceInformer:   serviceInformer,
	})
	if err != nil {
		return nil, err
	}

	firewall := firewall.NewFirewallController(pNode.Name, serviceInformer, endpointsInformer)
	go firewall.Run(ctx)

	kubeApi, err := kubeapi.NewKubeApi(kubernetes, podManager, pNode.Name, nodeIP)
	if err != nil {
		return nil, err
	}
	go kubeApi.Run(ctx)

	autoUpgrade, err := autoupgrade.NewAutoUpgrade(configMapInformer)
	if err != nil {
		return nil, err
	}
	go autoUpgrade.Run(ctx)

	go nodeRunner.Run(ctx)
	// err = nodeRunner.Run(ctx)
	// if err != nil {
	// 	panic(err)
	// }
	// log.Println("RUnning...")

	go podRunner.Run(ctx, 1) // number of sync workers
	log.Println("Starting...")
	<-nodeRunner.Ready()
	log.Println("Node runner ready")
	podInformerFactory.Start(ctx.Done())
	scmInformerFactory.Start(ctx.Done())
	log.Println("Informers started")

	kubeletEvents.Eventf(nodeRef, corev1.EventTypeNormal, "Starting" /*StartingKubelet*/, "Starting kube-pet-node.")

	return &PetNode{
		NodeName:   nodeName,
		Kubernetes: kubernetes,
		PodManager: podManager,

		NodeRunner: nodeRunner,
		PodRunner:  podRunner,
		Firewall:   firewall,

		PodInformer:       podInformer,
		SecretInformer:    secretInformer,
		ConfigMapInformer: configMapInformer,
		ServiceInformer:   serviceInformer,
	}, nil

	// select {
	// case <-podRunner.Ready():
	// 	log.Println("Ready...")
	// 	<-podRunner.Done()
	// 	log.Println("Done!")
	// case <-podRunner.Done():
	// 	log.Println("Done...")
	// }
	// if podRunner.Err() != nil {
	// 	log.Println(podRunner.Err())
	// 	// handle error
	// }
	// log.Println("exit")
}
