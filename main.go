package main

import (
	"log"

	"github.com/danopia/kube-edge-node/controller"
	"github.com/danopia/kube-edge-node/podman"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// type NP struct {}
// func (np *NP) NotifyNodeStatus(	)

func main() {

	// podman := podman.NewPodmanClient("unix", "/run/user/1000/podman/podman.sock")
	podman := podman.NewPodmanClient("tcp", "127.0.0.1:8410")

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", "node-kubeconfig.yaml")
	if err != nil {
		panic(err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	edgeNode, err := controller.NewEdgeNode("pet-berbox", podman, clientset)
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
