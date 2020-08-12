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

	"github.com/danopia/kube-pet-node/podman"
)

// PetNodeProvider is a node provider that fills in
// the status and health for our Kubernetes Node object.
type PetNodeProvider struct {
	node        *corev1.Node
	nodeStatus  *corev1.NodeStatus
	externalIpC <-chan string
}

func NewPetNodeProvider(node *corev1.Node, conVersion *podman.DockerVersionReport, maxPods int, nodeIP net.IP) (*PetNodeProvider, error) {

	log.Println("NodeIdentity: Building initial node status...")

	machineId, err := ioutil.ReadFile("/etc/machine-id")
	if err != nil {
		return nil, err
	}
	machineIdStr := strings.Trim(string(machineId), "\n")

	bootId, err := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return nil, err
	}
	bootIdStr := strings.Trim(string(bootId), "\n")

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
			"cpu":               resource.MustParse("1000m"),
			"memory":            resource.MustParse("1000Mi"),
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
			{
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Hello World",
				Reason:             "OK",
				Status:             "False",
				Type:               "NetworkUnavailable",
			},
		},
		NodeInfo: corev1.NodeSystemInfo{
			Architecture:            conVersion.Arch,
			BootID:                  bootIdStr,
			MachineID:               machineIdStr,
			KernelVersion:           conVersion.KernelVersion,
			OSImage:                 osPrettyName,
			ContainerRuntimeVersion: "podman://" + conVersion.Version,
			KubeletVersion:          "kube-pet/v0.1.0",     // TODO?
			KubeProxyVersion:        "nftables-pet/v0.0.1", // TODO?
			OperatingSystem:         conVersion.Os,
		},
		Addresses: []corev1.NodeAddress{
			{
				Type:    corev1.NodeHostName,
				Address: node.ObjectMeta.Name,
			},
			{
				Type:    corev1.NodeInternalIP,
				Address: nodeIP.String(),
			},
			{
				Type:    corev1.NodeInternalDNS,
				Address: node.ObjectMeta.Name + ".local",
			},
			// { // TODO: look up on da.gd/ip or something
			// 	Type:    corev1.NodeExternalIP,
			// 	Address: "35.222.199.140",
			// },
			// also NodeExternalDNS
		},
		DaemonEndpoints: corev1.NodeDaemonEndpoints{
			KubeletEndpoint: corev1.DaemonEndpoint{
				Port: 10250,
			},
		},
	}

	ipC, readyC := WatchInternetV4Address()
	select {
	case <-readyC:
	case <-time.After(5 * time.Second):
		log.Println("NodeIdentity: timeout waiting for our Internet IP address")
	}

	return &PetNodeProvider{
		node:        node,
		nodeStatus:  nodeStatus,
		externalIpC: ipC,
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

			case externalIp := <-np.externalIpC:
				matched := false
				for idx := range np.nodeStatus.Addresses {
					if np.nodeStatus.Addresses[idx].Type == corev1.NodeExternalIP {
						matched = true
						np.nodeStatus.Addresses[idx].Address = externalIp
						log.Println("NodeIdentity: Updated ExternalIP in our status")
					}
				}
				if !matched {
					np.nodeStatus.Addresses = append(np.nodeStatus.Addresses, corev1.NodeAddress{
						Type:    corev1.NodeExternalIP,
						Address: externalIp,
					})
					log.Println("NodeIdentity: Added ExternalIP to our status")
				}

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
