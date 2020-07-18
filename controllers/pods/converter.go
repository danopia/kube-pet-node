package pods

import (
	"log"
	"net"

	corev1 "k8s.io/api/core/v1"

	"github.com/danopia/kube-pet-node/podman"
)

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func ConvertPodConfig(pod *corev1.Pod, clusterDns net.IP, cniNet string) *podman.PodSpecGenerator {
	key := pod.ObjectMeta.Namespace + "_" + pod.ObjectMeta.Name

	shareNs := []string{"ipc", "net", "uts"}
	if pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
		shareNs = append(shareNs, "pid")
	}

	// TODO, probably: support rootless w/o pod IPs
	var netConfig podman.PodNetworkConfig
	if pod.Spec.HostNetwork {
		netConfig.NetNS.NSMode = "host"
	} else {
		netConfig.NetNS.NSMode = "bridge"
		netConfig.CNINetworks = []string{cniNet}
	}

	switch pod.Spec.DNSPolicy {
	case corev1.DNSClusterFirstWithHostNet:
		netConfig.DNSServer = []net.IP{clusterDns}
		netConfig.DNSSearch = []string{
			pod.ObjectMeta.Namespace + ".svc.cluster.local",
			"svc.cluster.local",
		}
		netConfig.DNSOption = []string{"ndots:5"}
	case corev1.DNSClusterFirst:
		if !pod.Spec.HostNetwork {
			netConfig.DNSServer = []net.IP{clusterDns}
			netConfig.DNSSearch = []string{
				pod.ObjectMeta.Namespace + ".svc.cluster.local",
				"svc.cluster.local",
			}
			netConfig.DNSOption = []string{"ndots:5"}
		}
	case corev1.DNSDefault: // TODO
	case corev1.DNSNone: // TODO
	}

	// TODO: all the port mappings?

	// pod spec fields, incomplete
	// TODO: volumes
	// TODO: InitContainers
	// TODO: EphemeralContainers
	// TODO: RestartPolicy (complex)
	// TODO: HostPID
	// TODO: HostIPC
	// TODO: SecurityContext
	// TODO: ImagePullSecrets
	// TODO: HostAliases
	// TODO: DNSConfig (easy)
	// TODO: SetHostnameAsFQDN (easy)

	return &podman.PodSpecGenerator{
		PodBasicConfig: podman.PodBasicConfig{
			Hostname:         pod.ObjectMeta.Name,
			Labels:           map[string]string{
				"heritage": "kube-pet-node",
			},
			Name:             key,
			SharedNamespaces: shareNs,
		},
		PodNetworkConfig: netConfig,
	}
}

func ConvertContainerConfig(pod *corev1.Pod, conSpec *corev1.Container, podId string) *podman.SpecGenerator {
	key := pod.ObjectMeta.Namespace + "_" + pod.ObjectMeta.Name

	conEnv := map[string]string{}
	for _, envVar := range conSpec.Env {
		if envVar.ValueFrom == nil {
			conEnv[envVar.Name] = envVar.Value
		} else {
			log.Println("WARN:", key, conSpec.Name, "env", envVar.Name, "is dynamic!")
			log.Printf("TODO: EnvVariable definition: %+v", envVar)
			conEnv[envVar.Name] = "TODO"
		}
	}

	var isSystemd string
	if value, ok := pod.ObjectMeta.Annotations["vk.podman.io/systemd."+conSpec.Name]; ok {
		isSystemd = value
	}

	return &podman.SpecGenerator{
		ContainerBasicConfig: podman.ContainerBasicConfig{
			Name:       key + "_" + conSpec.Name,
			Namespace:  "kube-pet",
			Pod:        podId, // creation.Id,
			Entrypoint: conSpec.Command,
			Command:    conSpec.Args,
			Env:        conEnv,
			Terminal:   conSpec.TTY,
			Stdin:      conSpec.Stdin,
			Labels: map[string]string{
				"k8s-name": conSpec.Name,
				"k8s-type": "standard", // vs init or ephemeral
			},
			Annotations: map[string]string{},
			// Annotations map[string]string `json:"annotations,omitempty"`
			// StopSignal *syscall.Signal `json:"stop_signal,omitempty"`
			// StopTimeout *uint `json:"stop_timeout,omitempty"`
			LogConfiguration: &podman.LogConfig{
				Driver: "k8s-file",
			},
			// RestartPolicy string `json:"restart_policy,omitempty"`
			// RestartRetries *uint `json:"restart_tries,omitempty"`
			// OCIRuntime string `json:"oci_runtime,omitempty"`
			Systemd: isSystemd,
			// Namespace string `json:"namespace,omitempty"`
			// PidNS Namespace `json:"pidns,omitempty"`
			// UtsNS Namespace `json:"utsns,omitempty"`
			// Hostname string `json:"hostname,omitempty"`
			// Sysctl map[string]string `json:"sysctl,omitempty"`
			// Remove bool `json:"remove,omitempty"`
			// PreserveFDs uint `json:"-"`
		},
		ContainerStorageConfig: podman.ContainerStorageConfig{
			Image: conSpec.Image,
			// ImageVolumeMode string `json:"image_volume_mode,omitempty"`
			// Mounts []Mount `json:"mounts,omitempty"`
			// Volumes []*NamedVolume `json:"volumes,omitempty"`
			// Devices []LinuxDevice `json:"devices,omitempty"`
			// IpcNS Namespace `json:"ipcns,omitempty"`
			// ShmSize *int64 `json:"shm_size,omitempty"`
			WorkDir: conSpec.WorkingDir,
			// RootfsPropagation string `json:"rootfs_propagation,omitempty"`
		},

		// TODO: ContainerSecurityConfig
		// TODO: ContainerResourceConfig
	}

	// container spec, exhasutive as of july 2020
	// Name
	// Image
	// Command
	// Args
	// WorkingDir
	// TODO: Ports
	// TODO: EnvFrom
	// Env
	// TODO: Resources
	// TODO: VolumeMounts
	// TODO: VolumeDevices
	// TODO: LivenessProbe
	// TODO: ReadinessProbe
	// TODO: StartupProbe
	// TODO: Lifecycle
	// TODO: TerminationMessagePath
	// TODO: TerminationMessagePolicy
	// TODO: ImagePullPolicy
	// TODO: SecurityContext
	// Stdin
	// TODO: StdinOnce
	// TTY
}
