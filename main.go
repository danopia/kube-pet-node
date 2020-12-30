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
	"github.com/danopia/kube-pet-node/controllers/pods"
	"github.com/danopia/kube-pet-node/controllers/selfprovision"
	"github.com/danopia/kube-pet-node/pkg/podman"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {

	var nameFlag = flag.String("hostname", "", "name to use for the Kubernetes node, will get prefixed with 'pet-'")
	var kubeconfFlag = flag.String("kubeconfig-path", "node-kubeconfig.yaml", "path to client config with a system:node clusterrolebinding")
	var podmanSockFlag = flag.String("podman-socket", "tcp:127.0.0.1:8410", "podman socket location, either 'tcp:' or 'unix:' prefix")
	var vpnIfaceFlag = flag.String("vpn-iface", "wg-gke", "network interface which the other cluster nodes and pods are available on")
	var cniNetFlag = flag.String("cni-net", "kube-pet-net", "CNI network which provides local pods with networking and addresses")
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
	nodeIP, err := fetchNodeAddressFromInterface(*vpnIfaceFlag)
	if err != nil {
		log.Println("WARN: failed to find Node IP from", *vpnIfaceFlag, ":", err)
	} else {
		log.Println("Discovered node IP:", nodeIP)
	}

	// unix:/run/user/1000/podman/podman.sock
	// tcp:127.0.0.1:8410
	podSockParts := strings.SplitN(*podmanSockFlag, ":", 2)
	podman := podman.NewPodmanClient(podSockParts[0], podSockParts[1])

	podStorage, err := pods.NewPodSpecStorage()
	if err != nil {
		panic(err)
	}

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

	// actually set up our lifespan now that we've loaded enough deps
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
		<-sig
		log.Fatalln("WARN: Received second signal -- bailing without shutdown!")
	}()

	// sync our local pod info sources
	podManager, err := pods.NewPodManager(podman, podStorage)
	if err != nil {
		panic(err)
	}

	// discover CIDRs that pods can use
	var podNets []net.IPNet
	maxPods := *maxPodsFlag
	cniNetworks, err := podman.NetworkInspect(ctx, *cniNetFlag)
	if err != nil {
		log.Println("WARN: failed to read CNI", *cniNetFlag, ":", err)
	}
	for _, cniNetwork := range cniNetworks {
		for _, plugin := range cniNetwork.Plugins {
			if plugin.Ipam != nil {
				// log.Printf("%+v", plugin.Ipam)
				if plugin.Ipam.Subnet != "" {
					_, ipn, err := net.ParseCIDR(plugin.Ipam.Subnet)
					if err != nil {
						panic(err)
					}
					podNets = append(podNets, *ipn)
				} else {
					log.Println("TODO: cni plugin IPAM without a Subnet")
				}
			}
		}
	}
	if len(podNets) < 1 && maxPods > 0 {
		log.Println("WARN: I couldn't discover any pod networks! I'm going to refuse to run any pods.")
		maxPods = 0
	}
	log.Println("Pod networks:", podNets)

	// construct the node
	petNode, err := controller.NewPetNode(ctx, nodeName, podManager, clientset, maxPods, *vpnIfaceFlag, nodeIP, podNets, *cniNetFlag)
	if err != nil {
		panic(err)
	}

	if petNode.PodRunner != nil {
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

	} else {

		selfProvisioner := selfprovision.NewController(nodeName, *vpnIfaceFlag, *cniNetFlag, clientset, podman)
		if err := selfProvisioner.Perform(ctx); err != nil {
			panic(err)
		}

	}

	log.Println("exit")
}

func fetchNodeAddressFromInterface(ifaceName string) (net.IP, error) {
	vpnIface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}
	vpnAddrs, err := vpnIface.Addrs()
	if err != nil {
		return nil, err
	}

	// retrieve node address from the VPN info
	for _, addr := range vpnAddrs {
		if net, ok := addr.(*net.IPNet); ok {
			// log.Println(net, net.IP.IsGlobalUnicast())
			if net.IP.IsGlobalUnicast() {
				ones, bits := net.Mask.Size()
				if ones == bits {
					// a single addr, treat as node address
					return net.IP, nil
				} else {
					log.Println("Skipping CIDR Addr", addr.String(), "on vpn interface")
				}
			}
		} else {
			log.Println("Skipping weird Addr", addr.String(), "on vpn interface")
		}
	}

	return nil, nil
}
