package main

import (
	"log"

	"github.com/danopia/kube-pet-node/controller"
	"github.com/danopia/kube-pet-node/podman"

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

	petNode, err := controller.NewPetNode("pet-berbox", podman, clientset)
	if err != nil {
		panic(err)
	}

	select {
	case <-petNode.PodRunner.Ready():
		log.Println("Ready...")
		<-petNode.PodRunner.Done()
		log.Println("Done!")
	case <-petNode.PodRunner.Done():
		log.Println("Done...")
	}
	if petNode.PodRunner.Err() != nil {
		log.Println(petNode.PodRunner.Err())
		// handle error
	}
	log.Println("exit")
}
