package main

import (
	// "context"
	"log"
	// "runtime"
	// "path"

	"github.com/danopia/kube-edge-node/controller"
	"github.com/danopia/kube-edge-node/podman"

	// "github.com/pbnjay/memory"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// corev1 "k8s.io/api/core/v1"
	// // coordv1 "k8s.io/api/coordination/v1beta1"
	// "k8s.io/apimachinery/pkg/api/resource"
	// corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	// // corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	// kubeinformers "k8s.io/client-go/informers"
	// "k8s.io/apimachinery/pkg/fields"
	// "k8s.io/client-go/kubernetes/scheme"
	// "k8s.io/client-go/tools/record"
	// "github.com/virtual-kubelet/virtual-kubelet/node"
	// // "github.com/virtual-kubelet/virtual-kubelet/log"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// "github.com/danopia/kube-edge-node/podman"
)

// type NP struct {}
// func (np *NP) NotifyNodeStatus(	)

func main() {

	podman := podman.NewPodmanClient("/run/user/1000/podman/podman.sock")

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", "/home/dan/.kube/config")
	if err != nil {
		panic(err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	edgeNode, err := controller.NewEdgeNode("phynode", podman, clientset)
	if err != nil {
		panic(err)
	}

	select {
	case <-edgeNode.PodRunner.Ready():
		log.Println("Ready...")
		<-edgeNode.PodRunner.Done()
		log.Println("Done!")
	case <-edgeNode.PodRunner.Done():
		log.Println("Done...")
	}
	if edgeNode.PodRunner.Err() != nil {
		log.Println(edgeNode.PodRunner.Err())
		// handle error
	}
	log.Println("exit")
}
