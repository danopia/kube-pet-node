package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	listers_corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/danopia/kube-pet-node/podman"
)

type PodmanProvider struct {
	podman  *podman.PodmanClient
	manager *PodManager
	events  record.EventRecorder
	secrets listers_corev1.SecretLister
	cniNet  string
	// pods        map[string]*corev1.Pod
	podNotifier func(*corev1.Pod)
	// specStorage *PodSpecStorage
}

func NewPodmanProvider(podManager *PodManager, events record.EventRecorder, secrets listers_corev1.SecretLister, cniNet string) *PodmanProvider {
	return &PodmanProvider{
		podman:  podManager.podman,
		manager: podManager,
		events:  events,
		secrets: secrets,
		cniNet:  cniNet,
		// pods:        make(map[string]*corev1.Pod),
		podNotifier: func(*corev1.Pod) {},
		// specStorage: specStorage,
	}
}

func (d *PodmanProvider) GrabPullSecrets(namespace string, secretRefs []corev1.LocalObjectReference) ([]*corev1.Secret, error) {
	lister := d.secrets.Secrets(namespace)
	secrets := make([]*corev1.Secret, len(secretRefs))
	for idx, ref := range secretRefs {
		secret, err := lister.Get(ref.Name)
		if err != nil {
			return secrets, err
		}
		secrets[idx] = secret
	}
	return secrets, nil
}

func (d *PodmanProvider) PullImage(ctx context.Context, imageRef string, pullSecrets []*corev1.Secret, podRef *corev1.ObjectReference) error {
	d.events.Eventf(podRef, corev1.EventTypeNormal, "Pulling", "Pulling image \"%s\"", imageRef)

	imgIds, err := d.podman.Pull(ctx, imageRef, nil)
	if err == nil {
		log.Println("Pulled images", imgIds, "for", podRef.Namespace, "/", podRef.Name)
		d.events.Eventf(podRef, corev1.EventTypeNormal, "Pulled", "Successfully pulled public image \"%s\"", imageRef)
		return nil
	} else if !strings.Contains(err.Error(), "unauthorized") {
		return err
	}

	for _, secret := range pullSecrets {
		if secret.Type != "kubernetes.io/dockerconfigjson" {
			return fmt.Errorf("TODO: ImagePullSecret %s has weird type %s", secret.ObjectMeta.Name, secret.Type)
		}

		dockerconfjson, ok := secret.Data[".dockerconfigjson"]
		if !ok {
			return fmt.Errorf("TODO: ImagePullSecret %s is missing .dockerconfigjson", secret.ObjectMeta.Name)
		}

		// round-trip json to pull out a specific field
		var dockerconf DockerConfigJson
		if err := json.Unmarshal(dockerconfjson, &dockerconf); err != nil {
			return err // TODO: wrap
		}
		authBody, jerr := json.Marshal(&dockerconf.Auths)
		if jerr != nil {
			return jerr // TODO: wrap
		}

		imgIds, err = d.podman.Pull(ctx, imageRef, authBody)
		if err == nil {
			log.Println("Pulled PRIVATE images", imgIds, "for", podRef.Namespace, "/", podRef.Name)
			d.events.Eventf(podRef, corev1.EventTypeNormal, "Pulled", "Successfully pulled private image \"%s\"", imageRef)
			return nil
		} else if !strings.Contains(err.Error(), "unauthorized") {
			return err
		}
	}

	return err
}

type DockerConfigJson struct {
	Auths map[string]DockerCredential
}
type DockerCredential struct {
	Username string
	Password string
	Email    string
	Auth     string
}

// CreatePod takes a Kubernetes Pod and deploys it within the provider.
func (d *PodmanProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	podCoord, err := d.manager.RegisterPod(pod)
	if err != nil {
		return err
	}
	log.Println("Pod", podCoord, "registered")

	podRef := &corev1.ObjectReference{
		APIVersion:      "v1",
		Kind:            "Pod",
		Namespace:       pod.ObjectMeta.Namespace,
		Name:            pod.ObjectMeta.Name,
		UID:             pod.ObjectMeta.UID,
		ResourceVersion: pod.ObjectMeta.ResourceVersion,
		// FieldPath:        "spec.containers{app}",
	}

	dnsServer := net.ParseIP("10.6.0.10") // TODO
	creation, err := d.podman.PodCreate(ctx, ConvertPodConfig(pod, dnsServer, d.cniNet))
	if err != nil {
		log.Println("pod create err", err)
		return err
	}
	d.manager.SetPodId(podCoord, creation.Id)

	pullSecrets, err := d.GrabPullSecrets(podRef.Namespace, pod.Spec.ImagePullSecrets)
	if err != nil {
		log.Println("TODO: image pull secrets lookup err:", err)
		return err
	}

	for _, conSpec := range pod.Spec.Containers {

		// Always pull first for Always
		if conSpec.ImagePullPolicy == corev1.PullAlways {
			err = d.PullImage(ctx, conSpec.Image, pullSecrets, podRef)
			if err != nil {
				log.Println("TODO: image pull", conSpec.Image, "err", err)
				return err
			}
		}

		conCreation, err := d.podman.ContainerCreate(ctx, ConvertContainerConfig(pod, &conSpec, creation.Id))
		if err != nil {

			// Pull on-the-spot for IfNotPresent
			if strings.HasSuffix(err.Error(), "no such image") && conSpec.ImagePullPolicy == corev1.PullIfNotPresent {

				err := d.PullImage(ctx, conSpec.Image, pullSecrets, podRef)
				if err != nil {
					log.Println("TODO: image pull", conSpec.Image, "err", err)
					return err
				}

				// ... and retry creation
				conCreation, err = d.podman.ContainerCreate(ctx, ConvertContainerConfig(pod, &conSpec, creation.Id))
				if err != nil {
					log.Println("container create err", err)
					return err
				}

			} else {
				log.Println("container create err", err)
				return err
			}
		}
		log.Printf("container create %+v", conCreation)
		d.events.Eventf(podRef, corev1.EventTypeNormal, "Created", "Created container %s", conSpec.Name)
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

	for idx, _ := range pod.Status.Conditions {
		cond := &pod.Status.Conditions[idx]
		if cond.Type == corev1.PodReady {
			cond.Status = corev1.ConditionTrue
		}
	}
	for idx, _ := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[idx]
		if insp, ok := containerInspects[cs.Name]; ok {
			cs.Ready = true
			cs.RestartCount = insp.RestartCount
			cs.ContainerID = "podman://" + insp.ID
			cs.ImageID = strings.Split(insp.ImageName, ":")[0] + "@shasum:" + insp.Image
			cs.State = corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{ // TODO: check!
					StartedAt: now,
				},
			}
			d.events.Eventf(podRef, corev1.EventTypeNormal, "Started", "Started container %s", cs.Name)
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
		if err, ok := err.(*podman.ApiError); ok {
			if err.Status == 404 {
				// pod already doesn't exist; so just clean up
				return d.manager.UnregisterPod(PodCoord{pod.ObjectMeta.Namespace, pod.ObjectMeta.Name})
			}
		}

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
	return d.manager.KnownPods[key].Kube, nil
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
	for _, pod := range d.manager.KnownPods {
		pods = append(pods, pod.Kube)
	}
	return pods, nil
}

func (d *PodmanProvider) NotifyPods(ctx context.Context, notifier func(*corev1.Pod)) {
	d.podNotifier = notifier
}
