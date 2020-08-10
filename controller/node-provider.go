package controller

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
	node       *corev1.Node
	conVersion *podman.DockerVersionReport
	maxPods    resource.Quantity
	nodeIP     net.IP
}

func NewPetNodeProvider(node *corev1.Node, conVersion *podman.DockerVersionReport, maxPods int, nodeIP net.IP) (*PetNodeProvider, error) {
	return &PetNodeProvider{
		node:       node,
		conVersion: conVersion,
		maxPods:    resource.MustParse(strconv.Itoa(maxPods)),
		nodeIP:     nodeIP,
	}, nil
}

func (np *PetNodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (np *PetNodeProvider) NotifyNodeStatus(ctx context.Context, f func(*corev1.Node)) {
	log.Println("Building node status...")

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

	machineId, err := ioutil.ReadFile("/etc/machine-id")
	if err != nil {
		return //nil, err
	}
	machineIdStr := strings.Trim(string(machineId), "\n")

	bootId, err := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return //nil, err
	}
	bootIdStr := strings.Trim(string(bootId), "\n")

	osPrettyName, err := findosPrettyName("/etc/os-release")
	if err != nil {
		return //nil, err
	}

	np.node.Status = corev1.NodeStatus{
		Capacity: corev1.ResourceList{
			"cpu":               *resource.NewScaledQuantity(int64(runtime.NumCPU()), 0),
			"memory":            *resource.NewQuantity(int64(memory.TotalMemory()), resource.BinarySI),
			"pods":              np.maxPods,
			"ephemeral-storage": resource.MustParse("10Gi"), // TODO
			"hugepages-2Mi":     resource.MustParse("0"),
		},
		Allocatable: corev1.ResourceList{
			"cpu":               resource.MustParse("1000m"),
			"memory":            resource.MustParse("1000Mi"),
			"pods":              np.maxPods,
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
		Images: localImagesMapped,
		NodeInfo: corev1.NodeSystemInfo{
			Architecture:            np.conVersion.Arch,
			BootID:                  bootIdStr,
			MachineID:               machineIdStr,
			KernelVersion:           np.conVersion.KernelVersion,
			OSImage:                 osPrettyName,
			ContainerRuntimeVersion: "podman://" + np.conVersion.Version,
			KubeletVersion:          "kube-pet/v0.1.0",     // TODO?
			KubeProxyVersion:        "nftables-pet/v0.0.1", // TODO?
			OperatingSystem:         np.conVersion.Os,
		},
		Addresses: []corev1.NodeAddress{
			{
				Type:    corev1.NodeHostName,
				Address: np.node.ObjectMeta.Name,
			},
			{
				Type:    corev1.NodeInternalIP,
				Address: np.nodeIP.String(),
			},
			{
				Type:    corev1.NodeInternalDNS,
				Address: np.node.ObjectMeta.Name + ".local",
			},
			// { // TODO: look up on da.gd/ip or something
			// 	Type:    corev1.NodeExternalIP,
			// 	Address: "35.222.199.140",
			// },
			// also NodeExternalDNS
		},
	}

	// status:
	//   daemonEndpoints:
	//     kubeletEndpoint:
	//       Port: 10250
	//   volumesAttached: []
	//   volumesInUse: []

	go f(np.node)
	log.Println("Node status updated!")
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
