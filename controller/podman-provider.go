package controller

import (
	"context"
	"log"
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

	"github.com/danopia/kube-edge-node/podman"
)

type PodmanProvider struct {
	podman      *podman.PodmanClient
	pods        map[string]*corev1.Pod
	podNotifier func(*corev1.Pod)
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (d *PodmanProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	key := pod.ObjectMeta.Namespace + "_" + pod.ObjectMeta.Name

	log.Println("create", key)
	log.Printf("create pod %+v", pod)
	d.pods[key] = pod

	creation, err := d.podman.PodCreate(ctx, podman.PodSpecGenerator{
		// CniNetworks, DnsOption
		// DnsSearch: []string{pod.ObjectMeta.Namespace+".svc.cluster.local."},
		// DnsServer: []string{"10.6.0.10"}, // 10.8.0.131:53,10.8.0.131:53
		PodBasicConfig: podman.PodBasicConfig{
			Hostname: pod.ObjectMeta.Name,
			Labels:   map[string]string{},
			Name:     key,
		},
		// TODO: Host network, Host ports
		// TODO: share process namespace
	})
	if err != nil {
		log.Println("pod create err", err)
		return err
	}
	log.Printf("pod create %+v", creation)

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

	pod.Status = corev1.PodStatus{
		Phase:     corev1.PodRunning,
		HostIP:    "1.2.3.4",
		PodIP:     "5.6.7.8",
		StartTime: &now,
		Conditions: []corev1.PodCondition{
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
		},
	}
	for _, cs := range pod.Status.ContainerStatuses {
		cs.State = corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{
				StartedAt: now,
			},
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
		cs.State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{
				StartedAt:   cs.State.Running.StartedAt,
				FinishedAt:  now,
				ExitCode:    420,
				Reason:      "pod deleted",
				ContainerID: "podman://" + msg.Id,
			},
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
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testpod",
				Namespace: "default",
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
