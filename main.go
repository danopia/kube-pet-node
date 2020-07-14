package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/danopia/kube-pet-node/controller"
	"github.com/danopia/kube-pet-node/podman"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {

	var nameFlag = flag.String("hostname", "", "name to use for the Kubernetes node, will get prefixed with 'pet-'")
	var kubeconfFlag = flag.String("kubeconfig-path", "node-kubeconfig.yaml", "path to client config with a system:node clusterrolebinding")
	var podmanSockFlag = flag.String("podman-socket", "tcp:127.0.0.1:8410", "podman socket location, either 'tcp:' or 'unix:' prefix")
	var maxPodsFlag = flag.Int("max-pods", 10, "number of pods this node should support. 0 effectively disables scheduling")
	_ = flag.String("controllers", "firewall,podman", "which features to run")
	flag.Parse()

	var nodeName string

	// read the kubeconfig ourselves to see what our user is called in it
	kubeConfig, err := (&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfFlag}).Load()
	if err != nil {
		panic(err)
	}
	kubeCtx := kubeConfig.Contexts[kubeConfig.CurrentContext]
	if strings.HasPrefix(kubeCtx.AuthInfo, "system:node:") {
		nodeName = strings.TrimPrefix(kubeCtx.AuthInfo, "system:node:")
	}
	if *nameFlag != "" {
		nodeName = "pet-" + *nameFlag
	}
	if nodeName == "" {
		log.Fatalln("Hostname is required and also couldn't be detected from the kubeconfig, try passing --hostname=<xyz>")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	// unix:/run/user/1000/podman/podman.sock
	// tcp:127.0.0.1:8410
	podSockParts := strings.SplitN(*podmanSockFlag, ":", 2)
	podman := podman.NewPodmanClient(podSockParts[0], podSockParts[1])

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfFlag)
	if err != nil {
		panic(err)
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	petNode, err := controller.NewPetNode(ctx, nodeName, podman, clientset, *maxPodsFlag)
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
