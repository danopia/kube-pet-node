package pods

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"github.com/danopia/kube-pet-node/controllers/caching"
	"github.com/danopia/kube-pet-node/controllers/volumes"
	"github.com/danopia/kube-pet-node/pkg/podman"
)

type PodmanProvider struct {
	podman  *podman.PodmanClient
	manager *PodManager
	events  record.EventRecorder
	volumes *volumes.VolumesController
	caching *caching.Controller
	cniNet  string
	// pods        map[string]*corev1.Pod
	podNotifier func(*corev1.Pod)
	// specStorage *PodSpecStorage
}

func NewPodmanProvider(podManager *PodManager, caching *caching.Controller, volumes *volumes.VolumesController, events record.EventRecorder, cniNet string) *PodmanProvider {
	return &PodmanProvider{
		podman:  podManager.podman,
		manager: podManager,
		events:  events,
		volumes: volumes,
		caching: caching,
		cniNet:  cniNet,
		// pods:        make(map[string]*corev1.Pod),
		podNotifier: func(*corev1.Pod) {},
		// specStorage: specStorage,
	}
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (d *PodmanProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	if val, ok := pod.Annotations["kubernetes.io/config.source"]; ok && val == "file" {
		// A static pod, presumably from us; just ignore it for now
		log.Println("Pods: Received create for a static pod", pod.ObjectMeta.Name)
		return nil
	}

	now := metav1.NewTime(time.Now())
	pod.Status = corev1.PodStatus{
		// HostIP: d.manager.NodeIP, // TODO!
		Phase: "ContainerCreating", // TODO: is this correct? not spec'd, but is used by kubelet?
		Conditions: []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: now,
			},
			{
				Type:               corev1.PodInitialized,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: now,
			},
		},
	}
	d.podNotifier(pod)

	podCoord, err := d.manager.RegisterPod(pod)
	if err != nil {
		return err
	}
	// log.Println("Pods:", podCoord, "registered")

	podRef := &corev1.ObjectReference{
		APIVersion:      "v1",
		Kind:            "Pod",
		Namespace:       pod.ObjectMeta.Namespace,
		Name:            pod.ObjectMeta.Name,
		UID:             pod.ObjectMeta.UID,
		ResourceVersion: pod.ObjectMeta.ResourceVersion,
		// FieldPath:        "spec.containers{app}",
	}

	err = d.volumes.CreatePodVolumes(ctx, pod)
	if err != nil {
		log.Println("Pods: volumes create err", err)
		return err
	}

	dnsServer := net.ParseIP("10.6.0.10") // TODO!
	creation, err := d.podman.PodCreate(ctx, ConvertPodConfig(pod, dnsServer, d.cniNet))
	if err != nil {
		log.Println("Pods: pod create err", err)
		return err
	}
	d.manager.SetPodId(podCoord, creation.Id)

	// Start the container ID list - won't be complete for a bit though
	containerIDs := make(map[string]string, len(pod.Spec.Containers)+1)
	if podInsp, err := d.podman.PodInspect(ctx, creation.Id); err != nil {
		log.Println("Pods: initial pod insp err", err)
		return err
	} else {
		containerIDs["_infra"] = podInsp.InfraContainerID
	}

	pullSecrets, err := d.GrabPullSecrets(ctx, podRef.Namespace, pod.Spec.ImagePullSecrets)
	if err != nil {
		log.Println("Pods TODO: image pull secrets lookup err:", err)
		return err
	}

	for _, conSpec := range pod.Spec.Containers {

		// Always pull first for Always
		if conSpec.ImagePullPolicy == corev1.PullAlways {
			err = d.PullImage(ctx, conSpec.Image, pullSecrets, podRef)
			if err != nil {
				log.Println("Pods TODO: image pull", conSpec.Image, "err", err)
				return err
			}
		}

		creation, err := d.podman.ContainerCreate(ctx, ConvertContainerConfig(pod, &conSpec, creation.Id))
		if err != nil {

			// Pull on-the-spot for IfNotPresent
			if strings.HasSuffix(err.Error(), "no such image") && conSpec.ImagePullPolicy == corev1.PullIfNotPresent {

				err := d.PullImage(ctx, conSpec.Image, pullSecrets, podRef)
				if err != nil {
					// TODO: go into ImagePullBackoff
					log.Println("Pods TODO: image pull", conSpec.Image, "err", err)
					return err
				}

				// ... and retry creation
				creation, err = d.podman.ContainerCreate(ctx, ConvertContainerConfig(pod, &conSpec, creation.Id))
				if err != nil {
					log.Println("Pods: container create err", err)
					return err
				}

			} else {
				log.Println("Pods: container create err", err)
				return err
			}
		}

		// TODO: figure out what kinda stuff this would be
		for _, warning := range creation.Warnings {
			d.events.Eventf(podRef, corev1.EventTypeWarning, "CreationWarning", "Container %s: %s", conSpec.Name, warning)
		}

		// log.Printf("Pods: container create %+v", conCreation)
		d.events.Eventf(podRef, corev1.EventTypeNormal, "Created", "Created container %s", conSpec.Name)
		containerIDs[conSpec.Name] = creation.Id
	}

	// This is used elsewhere for stats, etc
	d.manager.SetContainerIDs(podCoord, containerIDs)

	for idx := range pod.Status.Conditions {
		cond := &pod.Status.Conditions[idx]
		if cond.Type == corev1.PodInitialized {
			cond.Status = corev1.ConditionTrue
			cond.LastTransitionTime = metav1.NewTime(time.Now())
		}
	}

	for _, container := range pod.Spec.Containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name:         container.Name,
			Image:        container.Image,
			Ready:        false,
			RestartCount: 0,
			ContainerID:  "podman://" + containerIDs[container.Name],
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

	startedTime := metav1.NewTime(time.Now())
	_, err = d.podman.PodStart(ctx, creation.Id)
	if err != nil {
		log.Println("Pods: pod start err", err)
		return err
	}
	// log.Printf("Pods: pod started %+v", msg)

	pod.Status.Phase = corev1.PodRunning
	d.podNotifier(pod)
	d.manager.RegisterPod(pod)

	containerInspects := make(map[string]*podman.InspectContainerData)
	for conName, conID := range containerIDs {
		conInsp, err := d.podman.ContainerInspect(ctx, conID, false)
		if err != nil {
			log.Println("Pods WARN: container insp err", err)
			continue
		}
		// log.Printf("con insp %+v", conInsp)
		containerInspects[conName] = conInsp

		if conName == "_infra" {
			// Infra container; probably fill in pod networking info
			if !pod.Spec.HostNetwork {
				if infraNetwork, ok := conInsp.NetworkSettings.Networks[d.cniNet]; ok {
					pod.Status.PodIP = infraNetwork.InspectBasicNetworkConfig.IPAddress
					pod.Status.PodIPs = []corev1.PodIP{{IP: pod.Status.PodIP}}
				}
			}
		}
	}

	// TODO: wait for probes before saying Ready
	// TODO: really need like a whole per-pod lifecycle goroutine or something.

	for idx := range pod.Status.Conditions {
		cond := &pod.Status.Conditions[idx]
		if cond.Type == corev1.PodReady {
			cond.Status = corev1.ConditionTrue
			cond.LastTransitionTime = metav1.NewTime(time.Now())
		}
	}
	for idx := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[idx]
		if insp, ok := containerInspects[cs.Name]; ok {
			cs.Ready = true
			cs.RestartCount = insp.RestartCount
			cs.ImageID = strings.Split(insp.ImageName, ":")[0] + "@shasum:" + insp.Image // TODO: try using registry's sha256
			// TODO!!! insp.State
			cs.State = corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{ // TODO: check!
					StartedAt: startedTime,
				},
			}
			d.events.Eventf(podRef, corev1.EventTypeNormal, "Started", "Started container %s", cs.Name)
		} else {
			log.Println("Pods Warn: failed to match container", cs.Name, "from", containerInspects)
		}
	}

	d.podNotifier(pod)
	d.manager.RegisterPod(pod)

	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (d *PodmanProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	log.Println("Pods: update", pod.ObjectMeta.Name)
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
// state, as well as the pod. DeletePod may be called multiple times for the same pod.
func (d *PodmanProvider) DeletePod(ctx context.Context, pod *corev1.Pod) error {

	if val, ok := pod.Annotations["kubernetes.io/config.source"]; ok && val == "file" {
		// A static pod, presumably from us; just ignore it for now
		log.Println("Pods: Received delete for a static pod", pod.ObjectMeta.Name)
		return nil
	}

	log.Println("Pods: delete", pod.ObjectMeta.Name)

	key := pod.ObjectMeta.Namespace + "_" + pod.ObjectMeta.Name

	now := metav1.NewTime(time.Now())
	msg, err := d.podman.PodStop(ctx, key)
	if err != nil {
		if err, ok := err.(*podman.ApiError); ok {
			if err.Status == 404 {
				// pod already doesn't exist; so just clean up
				return d.manager.UnregisterPod(PodCoord{pod.ObjectMeta.Namespace, pod.ObjectMeta.Name})
			}
		}

		log.Println("Pods: pod stop err", err)
		return err
	}
	// log.Printf("Pods: pod stopped %+v", msg)

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
		log.Println("Pods: pod unreg err", err)
		return err
	}

	_, err = d.podman.PodRm(ctx, key, false)
	if err != nil {
		log.Println("Pods: pod del err", err)
		return err
	}
	// log.Printf("Pods: pod deleteded %+v", rmMsg)

	err = d.volumes.CleanupVolumes(ctx, &pod.ObjectMeta)
	if err != nil {
		log.Println("Pods: volumes cleanup err", err)
		return err
	}

	return nil
}

// GetPod retrieves a pod by name from the provider (can be cached).
// The Pod returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	log.Println("Pods: get pod", namespace, name)

	key := namespace + "_" + name
	return d.manager.KnownPods[key].Kube, nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
// The PodStatus returned is expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	log.Println("Pods: get status", namespace, name)
	// TODO
	return &corev1.PodStatus{}, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
// The Pods returned are expected to be immutable, and may be accessed
// concurrently outside of the calling goroutine. Therefore it is recommended
// to return a version after DeepCopy.
func (d *PodmanProvider) GetPods(context.Context) ([]*corev1.Pod, error) {
	log.Println("Pods: list pods")

	pods := make([]*corev1.Pod, 0)
	for _, pod := range d.manager.KnownPods {
		pods = append(pods, pod.Kube)
	}
	return pods, nil
}

func (d *PodmanProvider) NotifyPods(ctx context.Context, notifier func(*corev1.Pod)) {
	d.podNotifier = notifier
}
