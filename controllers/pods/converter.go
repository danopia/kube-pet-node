package pods

import (
	"log"
	"net"

	corev1 "k8s.io/api/core/v1"

	"github.com/pbnjay/memory"

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
			Hostname: pod.ObjectMeta.Name,
			Labels: map[string]string{
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

	cpuShares := uint64(milliCPUToShares(conSpec.Resources.Requests.Cpu().MilliValue()))
	resources := &podman.LinuxResources{
		CPU: &podman.LinuxCPU{
			Shares: &cpuShares,
		},
	}
	if !conSpec.Resources.Limits.Memory().IsZero() {
		memoryLimit := conSpec.Resources.Limits.Memory().Value()
		resources.Memory = &podman.LinuxMemory{
			Limit: &memoryLimit,
		}
	}
	if !conSpec.Resources.Limits.Cpu().IsZero() {
		cpuPeriod := uint64(CpuPeriod)
		cpuQuota := milliCPUToQuota(conSpec.Resources.Limits.Cpu().MilliValue(), CpuPeriod)
		resources.CPU.Period = &cpuPeriod
		resources.CPU.Quota = &cpuQuota
	}
	// TODO: HugepageLimits = GetHugepageLimitsFromResources(container.Resources)
	oomScoreAdjust := GetContainerOOMScoreAdjust(pod, conSpec, int64(memory.TotalMemory()))

	mounts := make([]podman.Mount, 0)
	volumes := make([]*podman.NamedVolume, 0)

	for _, volMount := range conSpec.VolumeMounts {
		var volSource *corev1.Volume = nil
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == volMount.Name {
				volSource = &volume
				break
			}
			log.Println("WARN: VolumeMount", volMount.Name, "couldn't be correlated with a Volume")
		}
		if volSource != nil {

			if volSource.VolumeSource.HostPath != nil {
				// TODO: volSource.VolumeSource.HostPath.Type
				flag := "rw"
				if volMount.ReadOnly {
					flag = "ro"
				}
				flags := []string{flag}

				// https://github.com/containers/podman/blob/0d26a573e3cf8cc5baea84206a86cb83b433b6d5/pkg/util/mountOpts.go#L107
				if value, ok := pod.ObjectMeta.Annotations["vk.podman.io/volume-selinux."+volMount.Name]; ok {
					switch value {
					case "relabel-private":
						flags = append(flags, "Z")
					case "relabel-shared":
						flags = append(flags, "z")
					}
				}

				mounts = append(mounts, podman.Mount{
					Type:        "bind",
					Source:      volSource.VolumeSource.HostPath.Path + volMount.SubPath,
					Destination: volMount.MountPath,
					Options:     flags,
				})

				continue
			}

			log.Println("TODO: Volume", volMount.Name, "isn't supported!")
		}
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
			Mounts:  mounts,
			Volumes: volumes,
			// Devices []LinuxDevice `json:"devices,omitempty"`
			// IpcNS Namespace `json:"ipcns,omitempty"`
			// ShmSize *int64 `json:"shm_size,omitempty"`
			WorkDir: conSpec.WorkingDir,
			// RootfsPropagation string `json:"rootfs_propagation,omitempty"`
		},

		// TODO: ContainerSecurityConfig

		ContainerResourceConfig: podman.ContainerResourceConfig{
			ResourceLimits: resources,
			OOMScoreAdj:    &oomScoreAdjust,
		},

		ContainerCgroupConfig: podman.ContainerCgroupConfig{
			CgroupsMode: "enabled",
		},
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

// Via https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/qos/policy.go

const (
	// KubeletOOMScoreAdj int = -999 // used for the kubernetes control plane itself
	guaranteedOOMScoreAdj int = -998
	besteffortOOMScoreAdj int = 1000
)

func GetContainerOOMScoreAdjust(pod *corev1.Pod, container *corev1.Container, memoryCapacity int64) int {
	if *pod.Spec.Priority >= int32(2*1000000000) {
		return guaranteedOOMScoreAdj
	}

	// QOS is on pod status as of K8S 1.6.0 - https://github.com/kubernetes/kubernetes/pull/37968
	switch pod.Status.QOSClass {
	case corev1.PodQOSGuaranteed:
		return guaranteedOOMScoreAdj
	case corev1.PodQOSBestEffort:
		return besteffortOOMScoreAdj
	}

	memoryRequest := container.Resources.Requests.Memory().Value()
	oomScoreAdjust := 1000 - (1000*memoryRequest)/memoryCapacity
	// A guaranteed pod using 100% of memory can have an OOM score of 10. Ensure
	// that burstable pods have a higher OOM score adjustment.
	if int(oomScoreAdjust) < (1000 + guaranteedOOMScoreAdj) {
		return (1000 + guaranteedOOMScoreAdj)
	}
	// Give burstable pods a higher chance of survival over besteffort pods.
	if int(oomScoreAdjust) == besteffortOOMScoreAdj {
		return int(oomScoreAdjust - 1)
	}
	return int(oomScoreAdjust)
}

// Via https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/kuberuntime/helpers_linux.go

const (
	// Taken from lmctfy https://github.com/google/lmctfy/blob/master/lmctfy/controllers/cpu_controller.cc
	minShares     = 2
	sharesPerCPU  = 1024
	milliCPUToCPU = 1000

	// 100000 is equivalent to 100ms
	CpuPeriod    = int64(100000)
	minCpuPeriod = int64(1000)
)

// milliCPUToShares converts milliCPU to CPU shares
func milliCPUToShares(milliCPU int64) int64 {
	if milliCPU == 0 {
		// Return 2 here to really match kernel default for zero milliCPU.
		return minShares
	}
	// Conceptually (milliCPU / milliCPUToCPU) * sharesPerCPU, but factored to improve rounding.
	shares := (milliCPU * sharesPerCPU) / milliCPUToCPU
	if shares < minShares {
		return minShares
	}
	return shares
}

// milliCPUToQuota converts milliCPU to CFS quota and period values
func milliCPUToQuota(milliCPU int64, period int64) (quota int64) {
	// CFS quota is measured in two values:
	//  - cfs_period_us=100ms (the amount of time to measure usage across)
	//  - cfs_quota=20ms (the amount of cpu time allowed to be used across a period)
	// so in the above example, you are limited to 20% of a single CPU
	// for multi-cpu environments, you just scale equivalent amounts
	// see https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt for details
	if milliCPU == 0 {
		return
	}

	// we then convert your milliCPU to a value normalized over a period
	quota = (milliCPU * period) / milliCPUToCPU

	// quota needs to be a minimum of 1ms.
	if quota < minCpuPeriod {
		quota = minCpuPeriod
	}

	return
}
