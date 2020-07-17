package controller

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// // coordv1 "k8s.io/api/coordination/v1beta1"
	// "k8s.io/apimachinery/pkg/api/resource"
	// corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	// // corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	// kubeinformers "k8s.io/client-go/informers"
	// "k8s.io/apimachinery/pkg/fields"
	// "k8s.io/client-go/kubernetes/scheme"
	// "k8s.io/client-go/tools/record"
	// "github.com/virtual-kubelet/virtual-kubelet/node"
	// // "github.com/virtual-kubelet/virtual-kubelet/log"

	"github.com/danopia/kube-pet-node/podman"
)

type PodmanProvider struct {
	podman      *podman.PodmanClient
	pods        map[string]*corev1.Pod
	podNotifier func(*corev1.Pod)
}

func NewPodmanProvider(podman *podman.PodmanClient) *PodmanProvider {
	return &PodmanProvider{
		podman:      podman,
		pods:        make(map[string]*corev1.Pod),
		podNotifier: func(*corev1.Pod) {},
	}
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (d *PodmanProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	key := pod.ObjectMeta.Namespace + "_" + pod.ObjectMeta.Name

	log.Println("create", key)
	// log.Printf("create pod %+v", pod)
	d.pods[key] = pod

	shareNs := []string{"ipc", "net", "uts"}
	if pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
		shareNs = append(shareNs, "pid")
	}

	var netConfig podman.PodNetworkConfig
	if pod.Spec.HostNetwork {
		netConfig.NetNS.NSMode = "host"
	} else {
		netConfig.NetNS.NSMode = "bridge"
		netConfig.CNINetworks = []string{"kube-pet-net"}
	}

	switch pod.Spec.DNSPolicy {
	case corev1.DNSClusterFirstWithHostNet:
		netConfig.DNSServer = []net.IP{net.ParseIP("10.6.0.10")}
		netConfig.DNSSearch = []string{
			pod.ObjectMeta.Namespace + ".svc.cluster.local",
			"svc.cluster.local",
		}
		netConfig.DNSOption = []string{"ndots:5"}
	case corev1.DNSClusterFirst:
		if !pod.Spec.HostNetwork {
			netConfig.DNSServer = []net.IP{net.ParseIP("10.6.0.10")}
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

	creation, err := d.podman.PodCreate(ctx, podman.PodSpecGenerator{
		PodBasicConfig: podman.PodBasicConfig{
			Hostname:         pod.ObjectMeta.Name,
			Labels:           map[string]string{},
			Name:             key,
			SharedNamespaces: shareNs,
		},
		PodNetworkConfig: netConfig,
	})
	if err != nil {
		log.Println("pod create err", err)
		return err
	}
	log.Printf("pod create %+v", creation)

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

	for _, conSpec := range pod.Spec.Containers {

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

		conCreation, err := d.podman.ContainerCreate(ctx, &podman.SpecGenerator{
			ContainerBasicConfig: podman.ContainerBasicConfig{
				Name:       key + "_" + conSpec.Name,
				Pod:        creation.Id,
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
		})
		if err != nil {
			log.Println("container create err", err)
			return err
		}
		log.Printf("container create %+v", conCreation)

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

	now := metav1.NewTime(time.Now())
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodInitialized,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionFalse,
			},
			{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionTrue,
			},
		},
	}

	for _, container := range pod.Spec.Containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        false,
			RestartCount: 0,
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason:  "Initializing",
					Message: "Container will be started soon.",
				},
			},
		})
	}

	d.podNotifier(pod)

	msg, err := d.podman.PodStart(ctx, creation.Id)
	if err != nil {
		log.Println("pod start err", err)
		return err
	}
	log.Printf("pod started %+v", msg)

	pod.Status.Phase = corev1.PodRunning
	pod.Status.StartTime = &now

	d.podNotifier(pod)

	insp, err := d.podman.PodInspect(ctx, creation.Id)
	if err != nil {
		log.Println("pod insp err", err)
		return err
	}
	log.Printf("pod insp %+v", insp)

	infraInsp, err := d.podman.ContainerInspect(ctx, insp.InfraContainerID, false)
	if err != nil {
		log.Println("infra insp err", err)
		return err
	}
	log.Printf("infra insp %+v", insp)

	if !pod.Spec.HostNetwork {
		if infraNetwork, ok := infraInsp.NetworkSettings.Networks["kube-pet-net"]; ok {
			pod.Status.PodIP = infraNetwork.InspectBasicNetworkConfig.IPAddress
		}
	}
	// pod.Status.HostIP =    "1.2.3.4" // TODO

	containerInspects := make(map[string]*podman.InspectContainerData)
	for _, conIds := range insp.Containers {
		if conIds.ID == insp.InfraContainerID {
			continue
		}

		conInsp, err := d.podman.ContainerInspect(ctx, conIds.ID, false)
		if err != nil {
			log.Println("con insp err", err)
			continue
		}
		log.Printf("con insp %+v", insp)
		containerInspects[conInsp.Config.Labels["k8s-name"]] = conInsp
	}

	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodInitialized,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   corev1.PodScheduled,
			Status: corev1.ConditionTrue,
		},
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if insp, ok := containerInspects[cs.Name]; ok {
			cs.RestartCount = insp.RestartCount
			cs.ContainerID = "podman://" + insp.ID
			cs.State = corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{ // TODO: check!
					StartedAt: now,
				},
			}
		} else {
			log.Println("Warn: failed to match container", cs.Name, "from", containerInspects)
		}
	}

	d.podNotifier(pod)

	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (d *PodmanProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	log.Println("update", pod.ObjectMeta.Name)
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
// state, as well as the pod. DeletePod may be called multiple times for the same pod.
func (d *PodmanProvider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	log.Println("delete", pod.ObjectMeta.Name)

	key := pod.ObjectMeta.Namespace + "_" + pod.ObjectMeta.Name

	now := metav1.NewTime(time.Now())
	msg, err := d.podman.PodStop(ctx, key)
	if err != nil {
		log.Println("pod stop err", err)
		return err
	}
	log.Printf("pod stopped %+v", msg)

	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodInitialized,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.PodScheduled,
			Status: corev1.ConditionTrue,
		},
	}
	for _, cs := range pod.Status.ContainerStatuses {
		prevState := cs.State
		cs.State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				// StartedAt:   cs.State.Running.StartedAt,
				FinishedAt:  now,
				ExitCode:    420,
				Reason:      "pod deleted",
				ContainerID: "podman://" + msg.Id,
			},
		}
		if prevState.Running != nil {
			cs.State.Terminated.StartedAt = prevState.Running.StartedAt
		}
	}

	d.podNotifier(pod)

	delete(d.pods, key)

	rmMsg, err := d.podman.PodRm(ctx, key)
	if err != nil {
		log.Println("pod del err", err)
		return err
	}
	log.Printf("pod deleteded %+v", rmMsg)

	return nil
}

// GetPod retrieves a pod by name from the provider (can be cached).
// The Pod returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	log.Println("get pod", namespace, name)

	key := namespace + "_" + name
	return d.pods[key], nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	log.Println("get status", namespace, name)
	return &corev1.PodStatus{}, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPods(context.Context) ([]*corev1.Pod, error) {
	log.Println("list pods")

	sysPods, err := d.podman.PodPs(context.TODO())
	if err != nil {
		return nil, err
	}

	pods := make([]*corev1.Pod, 0)
	for _, sysPod := range sysPods {
		nameParts := strings.Split(sysPod.Name, "_")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nameParts[1],
				Namespace: nameParts[0],
				Labels:    sysPod.Labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: sysPod.Containers[0].Names,
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: "Running",
				PodIP: "8.8.8.8",
			},
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

func (d *PodmanProvider) NotifyPods(ctx context.Context, notifier func(*corev1.Pod)) {
	d.podNotifier = notifier
}
