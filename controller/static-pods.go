package controller

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (pn *PetNode) EnsureStaticPods(ctx context.Context) error {

	bootTime, err := getBootTime()
	if err != nil {
		return err
	}

	petPodAPI := pn.Kubernetes.CoreV1().Pods("kube-pets")
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "pet-host",
				Image: "none",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("0"),
						corev1.ResourceMemory: resource.MustParse("0Ki"),
					},
				},
			},
		},
		HostNetwork: true,
		NodeName:    pn.NodeName,
		// PriorityClassName: "system-node-critical", (must be in kube-system?)
		Tolerations: []corev1.Toleration{
			{
				Effect:   "NoExecute",
				Operator: "Exists",
			},
			{
				Effect:   "NoSchedule",
				Operator: "Exists",
			},
		},
	}
	podStatus := corev1.PodStatus{
		Conditions: []corev1.PodCondition{{
			Type:               "Ready",
			LastTransitionTime: metav1.NewTime(bootTime),
			Status:             "True",
		}},
		ContainerStatuses: []corev1.ContainerStatus{{
			Name:         "pet-host",
			Image:        "none",
			Started:      &[]bool{true}[0],
			Ready:        true,
			RestartCount: 0,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(bootTime),
				},
			},
		}},
		Phase:     corev1.PodRunning,
		StartTime: &[]metav1.Time{metav1.NewTime(bootTime)}[0],
	}

	if pod, err := petPodAPI.Get(ctx, "host-"+pn.NodeName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		log.Println("Static pod didn't exist yet :)")

		_, err := petPodAPI.Create(ctx, &corev1.Pod{
			// TypeMeta: metav1.TypeMeta{
			// 	APIVersion: "v1",
			// 	Kind:       "Pod",
			// },
			ObjectMeta: metav1.ObjectMeta{
				Name: "host-" + pn.NodeName,
				Labels: map[string]string{
					"component": "host",
					"tier":      "node",
				},
				Annotations: map[string]string{
					"kubernetes.io/config.source": "file",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Controller: &[]bool{true}[0],
						Kind:       "Node",
						Name:       pn.NodeName,
						UID:        pn.NodeUID,
					},
				},
			},
			Spec:   podSpec,
			Status: podStatus,
		}, metav1.CreateOptions{})
		return err

	} else {
		log.Println("When checking static pod:", err)

		_, err := petPodAPI.UpdateStatus(ctx, &corev1.Pod{
			ObjectMeta: pod.ObjectMeta,
			Spec:       podSpec,
			Status:     podStatus,
		}, metav1.UpdateOptions{})
		return err
	}
}

func getBootTime() (time.Time, error) {
	var sysinfo syscall.Sysinfo_t
	err := syscall.Sysinfo(&sysinfo)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not get boot time: %v", err)
	}
	// sysinfo only has seconds
	return time.Now().Add(-1 * (time.Duration(sysinfo.Uptime) * time.Second)), nil
}
