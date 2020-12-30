package nodeidentity

import (
	"bufio"
	"context"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pbnjay/memory"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/danopia/kube-pet-node/pkg/podman"
)

// TODO: IPv6 can't be enabled until virtual-kubelet fixes corruption

// PetNodeProvider is a node provider that fills in
// the status and health for our Kubernetes Node object.
type PetNodeProvider struct {
	node          *corev1.Node
	nodeStatus    *corev1.NodeStatus
	externalIPV4C <-chan string
	// externalIPV6C <-chan string
}

func NewPetNodeProvider(node *corev1.Node, petVersion string, conVersion *podman.DockerVersionReport, maxPods int, nodeIP net.IP) (*PetNodeProvider, error) {
	log.Println("NodeIdentity: Building initial node status...")

	machineID, err := ioutil.ReadFile("/etc/machine-id")
	if err != nil {
		return nil, err
	}
	machineIDStr := strings.Trim(string(machineID), "\n")

	bootID, err := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return nil, err
	}
	bootIDStr := strings.Trim(string(bootID), "\n")

	osPrettyName, err := findosPrettyName("/etc/os-release")
	if err != nil {
		return nil, err
	}

	nodeStatus := &corev1.NodeStatus{
		Capacity: corev1.ResourceList{
			"cpu":               *resource.NewScaledQuantity(int64(runtime.NumCPU()), 0),
			"memory":            *resource.NewQuantity(int64(memory.TotalMemory()), resource.BinarySI),
			"pods":              resource.MustParse(strconv.Itoa(maxPods)),
			"ephemeral-storage": resource.MustParse("10Gi"), // TODO
			"hugepages-2Mi":     resource.MustParse("0"),
		},
		Allocatable: corev1.ResourceList{
			"cpu":               *resource.NewScaledQuantity((int64(runtime.NumCPU()) * 90), -2),                       // allow 90% of the sytem
			"memory":            *resource.NewQuantity(int64(memory.TotalMemory())-(128*1024*1024), resource.BinarySI), // reserve 128Mi
			"pods":              resource.MustParse(strconv.Itoa(maxPods)),
			"ephemeral-storage": resource.MustParse("1Gi"), // TODO
			"hugepages-2Mi":     resource.MustParse("0"),
		},
		Conditions: []corev1.NodeCondition{
			{
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Hello World",
				Reason:             "KubeletReady",
				Status:             "True",
				Type:               "Ready",
			},
			{
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Hello World",
				Reason:             "OK",
				Status:             "False",
				Type:               "MemoryPressure",
			},
			{
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Hello World",
				Reason:             "OK",
				Status:             "False",
				Type:               "DiskPressure",
			},
			{
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Hello World",
				Reason:             "OK",
				Status:             "False",
				Type:               "PIDPressure",
			},
		},
		NodeInfo: corev1.NodeSystemInfo{
			Architecture:            conVersion.Arch,
			BootID:                  bootIDStr,
			MachineID:               machineIDStr,
			KernelVersion:           conVersion.KernelVersion,
			OSImage:                 osPrettyName,
			ContainerRuntimeVersion: "podman://" + conVersion.Version,
			KubeletVersion:          "kube-pet/" + petVersion,
			KubeProxyVersion:        "nftables-pet/" + petVersion,
			OperatingSystem:         conVersion.Os,
		},
		Addresses: []corev1.NodeAddress{
			{
				Type:    corev1.NodeHostName,
				Address: node.ObjectMeta.Name,
			},
			{
				Type:    corev1.NodeInternalDNS,
				Address: node.ObjectMeta.Name + ".local",
			},
			// also NodeExternalDNS maybe someday
		},
		DaemonEndpoints: corev1.NodeDaemonEndpoints{
			KubeletEndpoint: corev1.DaemonEndpoint{
				Port: 10250,
			},
		},
	}

	if len(nodeIP) > 0 {
		nodeStatus.Addresses = append(nodeStatus.Addresses, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: nodeIP.String(),
		})
		nodeStatus.Conditions = append(nodeStatus.Conditions, corev1.NodeCondition{
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            "Hello World",
			Reason:             "OK",
			Status:             "False",
			Type:               "NetworkUnavailable",
		})

	} else {
		nodeStatus.Conditions = append(nodeStatus.Conditions, corev1.NodeCondition{
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            "No node IP detected. Auto provisioning should fix this soon.",
			Reason:             "NoAddress",
			Status:             "True",
			Type:               "NetworkUnavailable",
		})
		// TODO: better way of driving the primary 'Ready' condition
		nodeStatus.Conditions[0].Status = "False"
		nodeStatus.Conditions[0].Reason = "NoAddress"
		nodeStatus.Conditions[0].Message = "No node IP detected"
	}

	ipV4C, readyV4C := WatchInternetAddress("4.da.gd")
	// ipV6C, readyV6C := WatchInternetAddress("6.da.gd")
	timeout := time.After(5 * time.Second)

	select {
	case externalV4IP := <-ipV4C:
		nodeStatus.Addresses = append(nodeStatus.Addresses, corev1.NodeAddress{
			Type:    corev1.NodeExternalIP,
			Address: externalV4IP,
		})
		log.Println("NodeIdentity: Added ExternalIP V4 to our status")
	case <-readyV4C:
	case <-timeout:
		log.Println("NodeIdentity: timeout waiting for our Internet IPv4 address")
	}

	// select {
	// case externalV6IP := <-ipV6C:
	// 	nodeStatus.Addresses = append(nodeStatus.Addresses, corev1.NodeAddress{
	// 		Type:    corev1.NodeExternalIP,
	// 		Address: externalV6IP,
	// 	})
	// 	log.Println("NodeIdentity: Added ExternalIP V6 to our status")
	// case <-readyV6C:
	// case <-timeout:
	// 	log.Println("NodeIdentity: timeout waiting for our Internet IPv6 address")
	// }

	return &PetNodeProvider{
		node:          node,
		nodeStatus:    nodeStatus,
		externalIPV4C: ipV4C,
		// externalIPV6C: ipV6C,
	}, nil
}

func (np *PetNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (np *PetNodeProvider) NotifyNodeStatus(ctx context.Context, f func(*corev1.Node)) {
	go func() {

		ticker := time.NewTicker(1 * time.Hour)
		firstRunC := make(chan struct{})
		close(firstRunC)

		for {
			select {

			case externalV4IP := <-np.externalIPV4C:
				matched := false
				for idx := range np.nodeStatus.Addresses {
					if np.nodeStatus.Addresses[idx].Type == corev1.NodeExternalIP {
						if strings.Contains(np.nodeStatus.Addresses[idx].Address, ".") {
							matched = true
							np.nodeStatus.Addresses[idx].Address = externalV4IP
							log.Println("NodeIdentity: Updated ExternalIP V4 in our status")
						}
					}
				}
				if !matched {
					np.nodeStatus.Addresses = append(np.nodeStatus.Addresses, corev1.NodeAddress{
						Type:    corev1.NodeExternalIP,
						Address: externalV4IP,
					})
					log.Println("NodeIdentity: Added ExternalIP V4 to our status")
				}

			// case externalV6IP := <-np.externalIPV6C:
			// 	matched := false
			// 	for idx := range np.nodeStatus.Addresses {
			// 		if np.nodeStatus.Addresses[idx].Type == corev1.NodeExternalIP {
			// 			if strings.Contains(np.nodeStatus.Addresses[idx].Address, ":") {
			// 				matched = true
			// 				np.nodeStatus.Addresses[idx].Address = externalV6IP
			// 				log.Println("NodeIdentity: Updated ExternalIP V6 in our status")
			// 			}
			// 		}
			// 	}
			// 	if !matched {
			// 		np.nodeStatus.Addresses = append(np.nodeStatus.Addresses, corev1.NodeAddress{
			// 			Type:    corev1.NodeExternalIP,
			// 			Address: externalV6IP,
			// 		})
			// 		log.Println("NodeIdentity: Added ExternalIP V6 to our status")
			// 	}

			case <-ticker.C:
				log.Println("NodeIdentity: Performing periodic status refresh")
				// TODO: sorting, top 25
				// localImages, err := np.podman.List(ctx)
				// if err != nil {
				// 	return //nil, err
				// }
				var localImagesMapped []corev1.ContainerImage
				// for _, img := range localImages {
				// 	localImagesMapped = append(localImagesMapped, corev1.ContainerImage{
				// 		Names:     img.Names,
				// 		SizeBytes: img.Size,
				// 	})
				// }

				np.nodeStatus.Images = localImagesMapped

			case <-firstRunC:
				log.Println("NodeIdentity: Reporting initial NodeStatus")
			}
			firstRunC = nil

			// actually report
			newNode := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: np.node.Annotations,
					Labels:      np.node.Labels,
				},
			}
			np.nodeStatus.DeepCopyInto(&newNode.Status)
			f(newNode)
			log.Println("NodeIdentity: Node status updated!")
		}
	}()
}

func findosPrettyName(fname string) (string, error) {
	f, err := os.Open(fname[:])
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "PRETTY_NAME=") {
			return text[13 : len(text)-1], nil
		}
	}
	return "", scanner.Err()
}
