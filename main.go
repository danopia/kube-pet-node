package main

import (
	"context"
	"log"
	"path"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	// coordv1 "k8s.io/api/coordination/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	// corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	// "github.com/virtual-kubelet/virtual-kubelet/log"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// type NP struct {}
// func (np *NP) NotifyNodeStatus(	)

func main() {


	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", "/home/dan/.kube/config")
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}


	pNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "phynode",
			Labels: map[string]string{
				"purpose": "metal",
			},
		},
Spec: corev1.NodeSpec{
	PodCIDR: "10.6.2.33/27",
  PodCIDRs: []string{"10.6.2.33/27"},
  ProviderID: "metal://phynode",
  // taints:
  // - effect: PreferNoSchedule
  //   key: cloud.google.com/physical
  //   value: "false"
},
Status: corev1.NodeStatus{
	Capacity: corev1.ResourceList{
		"cpu": resource.MustParse("1000m"),
		"memory": resource.MustParse("1000Mi"),
		"pods": resource.MustParse("1"),
	},
	Allocatable: corev1.ResourceList{
		"cpu": resource.MustParse("1000m"),
		"memory": resource.MustParse("1000Mi"),
		"pods": resource.MustParse("1"),
	},
	Conditions: []corev1.NodeCondition{
		{
			// lastHeartbeatTime: "2020-06-30T17:20:59Z"
			// lastTransitionTime: "2020-05-18T22:36:38Z"
			Message: "Hello World",
			Reason: "KubeletReady",
			Status: "True",
			Type: "Ready",
		},
		{
			Message: "Hello World",
			Reason: "OK",
			Status: "False",
			Type: "MemoryPressure",
		},
		{
			Message: "Hello World",
			Reason: "OK",
			Status: "False",
			Type: "DiskPressure",
		},
		{
			Message: "Hello World",
			Reason: "OK",
			Status: "False",
			Type: "PIDPressure",
		},
	},
	NodeInfo: corev1.NodeSystemInfo{
		MachineID: "phynode",
		KernelVersion: "4.19.0-9-amd64",
		OSImage: "Debian GNU/Linux 10 (buster)",
		ContainerRuntimeVersion: "podman://1.9.3",
		KubeletVersion: "metal/v0.1.0",
		OperatingSystem: "Debian GNU/Linux 10 (buster)",
		Architecture: "amd64",
	},
	Addresses: []corev1.NodeAddress{
		{
			Type: corev1.NodeHostName,
			Address: "phynode",
		},
		{
			Type: corev1.NodeInternalIP,
			Address: "10.6.1.6",
		},
		{
			Type: corev1.NodeInternalDNS,
			Address: "phynode.local",
		},
		{
			Type: corev1.NodeExternalIP,
			Address: "8.8.8.8",
		},
	},
},
// status:
//   allocatable:
//     attachable-volumes-gce-pd: "127"
//     cpu: 940m
//     ephemeral-storage: "4278888833"
//     hugepages-2Mi: "0"
//     memory: 2700500Ki
//     pods: "50"
//   capacity:
//     attachable-volumes-gce-pd: "127"
//     cpu: "1"
//     ephemeral-storage: 16293736Ki
//     hugepages-2Mi: "0"
//     memory: 3785940Ki
//     pods: "50"
//   daemonEndpoints:
//     kubeletEndpoint:
//       Port: 10250
//   images: []
//   nodeInfo:
//     architecture: amd64
//     containerRuntimeVersion: containerd://1.2.8
//     kernelVersion: 4.19.109+
//     kubeProxyVersion: v1.16.8-gke.15
//     kubeletVersion: v1.16.8-gke.15
//     operatingSystem: linux
//     osImage: Container-Optimized OS from Google
//   volumesAttached: []
//   volumesInUse: []
	}

	nodeRunner, _ := node.NewNodeController(node.NaiveNodeProvider{}, pNode, clientset.CoreV1().Nodes(), node.WithNodeEnableLeaseV1Beta1(clientset.CoordinationV1beta1().Leases(corev1.NamespaceNodeLease), nil))


	//https://github.com/virtual-kubelet/virtual-kubelet/blob/3ec3b14e49d0c2f335ca049155d1ee94b2baf35f/cmd/virtual-kubelet/internal/commands/root/root.go

	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		clientset,
		60*1000000000,
		// kubeinformers.WithNamespace(c.KubeNamespace),
		kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", pNode.Name).String()
		}))
	podInformer := podInformerFactory.Core().V1().Pods()

	// Create another shared informer factory for Kubernetes secrets and configmaps (not subject to any selectors).
	scmInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(clientset, 60*1000000000)
	// Create a secret informer and a config map informer so we can pass their listers to the resource manager.
	secretInformer := scmInformerFactory.Core().V1().Secrets()
	configMapInformer := scmInformerFactory.Core().V1().ConfigMaps()
	serviceInformer := scmInformerFactory.Core().V1().Services()

	eb := record.NewBroadcaster()
	eb.StartLogging(func (a string, b ...interface {}) {
		log.Printf("record: %+v %+v", a, b)
	})
	// eb.StartLogging(log.G(context.TODO()).Infof)
	eb.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: clientset.CoreV1().Events("kube-system")})


	// setup other things
	podRunner, err := node.NewPodController(node.PodControllerConfig{
		PodClient: clientset.CoreV1(),
		Provider: &Dummy{},

		PodInformer:       podInformer,
		EventRecorder:     eb.NewRecorder(scheme.Scheme, corev1.EventSource{Component: path.Join(pNode.Name, "pod-controller")}),
		SecretInformer:    secretInformer,
		ConfigMapInformer: configMapInformer,
		ServiceInformer:   serviceInformer,
	})
	if err != nil {
		panic(err)
	}

	go podInformerFactory.Start(context.TODO().Done())
	go scmInformerFactory.Start(context.TODO().Done())

	go nodeRunner.Run(context.TODO())
	// err = nodeRunner.Run(context.TODO())
	// if err != nil {
	// 	panic(err.Error())
	// }
	// log.Println("RUnning...")

	go podRunner.Run(context.TODO(), 1)
	log.Println("RUnning...")

	select {
	case <-podRunner.Ready():
		log.Println("Ready...")
		<-podRunner.Done()
		log.Println("Done!")
	case <-podRunner.Done():
		log.Println("Done...")
	}
	if podRunner.Err() != nil {
		log.Println(podRunner.Err())
		// handle error
	}
	log.Println("exit")
}

type Dummy struct {

}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (d *Dummy) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	log.Println("create", pod.ObjectMeta.Name)
	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (d *Dummy) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	log.Println("update", pod.ObjectMeta.Name)
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
// state, as well as the pod. DeletePod may be called multiple times for the same pod.
func (d *Dummy) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	log.Println("delete", pod.ObjectMeta.Name)
	return nil
}

// GetPod retrieves a pod by name from the provider (can be cached).
// The Pod returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *Dummy) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	log.Println("get pod", namespace, name)
	return nil, nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *Dummy) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	log.Println("get status", namespace, name)
	return nil, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *Dummy) GetPods(context.Context) ([]*corev1.Pod, error) {
	log.Println("list pods")
	return nil, nil
}
