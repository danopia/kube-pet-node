package main

import (
	"context"
	"flag"
	"log"
	"net"
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
	var vpnIfaceFlag = flag.String("vpn-iface", "wg0", "network interface which the other cluster nodes and pods are available on")
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

	// look up our VPN information
	vpnIface, err := net.InterfaceByName(*vpnIfaceFlag)
	if err != nil {
		panic(err)
	}
	vpnAddrs, err := vpnIface.Addrs()
	if err != nil {
		panic(err)
	}

	// retrieve node address from the VPN info
	var nodeIP net.IP
	for _, addr := range vpnAddrs {
		if net, ok := addr.(*net.IPNet); ok {
			log.Println(net, net.IP.IsGlobalUnicast())
			if net.IP.IsGlobalUnicast() {
				ones, bits := net.Mask.Size()
				if ones == bits {
					// a single addr, treat as node address
					nodeIP = net.IP
				} else {
					log.Println("Skipping CIDR Addr", addr.String(), "on vpn interface")
				}
			}
		} else {
			log.Println("Skipping weird Addr", addr.String(), "on vpn interface")
		}
	}
	log.Println("Node IP:", nodeIP)

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

	petNode, err := controller.NewPetNode(ctx, nodeName, podman, clientset, *maxPodsFlag, nodeIP)
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
