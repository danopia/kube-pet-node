package pods

import (
	"context"
	"strings"
	"log"
	"net"
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
	// // "github.com/vi	rtual-kubelet/virtual-kubelet/log"

	"github.com/danopia/kube-pet-node/podman"
)

type PodmanProvider struct {
	podman      *podman.PodmanClient
	manager *PodManager
	cniNet      string
	// pods        map[string]*corev1.Pod
	podNotifier func(*corev1.Pod)
	// specStorage *PodSpecStorage
}

func NewPodmanProvider(podManager *PodManager, cniNet string) *PodmanProvider {
	return &PodmanProvider{
		podman:      podManager.podman,
		manager: podManager,
		cniNet:      cniNet,
		// pods:        make(map[string]*corev1.Pod),
		podNotifier: func(*corev1.Pod) {},
		// specStorage: specStorage,
	}
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (d *PodmanProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	podCoord, err := d.manager.RegisterPod(pod)
	if err != nil {
		return err
	}
	log.Println("Pod", podCoord, "registered")

	dnsServer := net.ParseIP("10.6.0.10") // TODO
	creation, err := d.podman.PodCreate(ctx, ConvertPodConfig(pod, dnsServer, d.cniNet))
	if err != nil {
		log.Println("pod create err", err)
		return err
	}
	log.Printf("pod create %+v", creation)

	for _, conSpec := range pod.Spec.Containers {
		conCreation, err := d.podman.ContainerCreate(ctx, ConvertContainerConfig(pod, &conSpec, creation.Id))
		if err != nil {
			log.Println("container create err", err)
			return err
		}
		log.Printf("container create %+v", conCreation)
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
	d.manager.RegisterPod(pod)

	msg, err := d.podman.PodStart(ctx, creation.Id)
	if err != nil {
		log.Println("pod start err", err)
		return err
	}
	log.Printf("pod started %+v", msg)

	pod.Status.Phase = corev1.PodRunning
	pod.Status.StartTime = &now

	d.podNotifier(pod)
	d.manager.RegisterPod(pod)

	insp, err := d.podman.PodInspect(ctx, creation.Id)
	if err != nil {
		log.Println("pod insp err", err)
		return err
	}
	// log.Printf("pod insp %+v", insp)

	infraInsp, err := d.podman.ContainerInspect(ctx, insp.InfraContainerID, false)
	if err != nil {
		log.Println("infra insp err", err)
		return err
	}
	// log.Printf("infra insp %+v", infraInsp)

	if !pod.Spec.HostNetwork {
		if infraNetwork, ok := infraInsp.NetworkSettings.Networks[d.cniNet]; ok {
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
		// log.Printf("con insp %+v", conInsp)
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
	for idx, _ := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[idx]
		if insp, ok := containerInspects[cs.Name]; ok {
			cs.Ready = true
			cs.RestartCount = insp.RestartCount
			cs.ContainerID = "podman://" + insp.ID
			cs.ImageID = strings.Split(insp.ImageName, ":")[0]+"@shasum:"+insp.Image
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
	d.manager.RegisterPod(pod)

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

	err = d.manager.UnregisterPod(PodCoord{pod.ObjectMeta.Namespace, pod.ObjectMeta.Name})
	if err != nil {
		log.Println("pod unreg err", err)
		return err
	}

	rmMsg, err := d.podman.PodRm(ctx, key, false)
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
	return d.manager.knownPods[key], nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	log.Println("get status", namespace, name)
	// TODO
	return &corev1.PodStatus{}, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPods(context.Context) ([]*corev1.Pod, error) {
	log.Println("list pods")

	pods := make([]*corev1.Pod, 0)
	for _, podSpec := range d.manager.knownPods {
		pods = append(pods, podSpec)
	}
	return pods, nil
}

func (d *PodmanProvider) NotifyPods(ctx context.Context, notifier func(*corev1.Pod)) {
	d.podNotifier = notifier
}
