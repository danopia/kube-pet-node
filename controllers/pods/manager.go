package pods

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/danopia/kube-pet-node/podman"
)

type PodManager struct {
	podman      *podman.PodmanClient
	specStorage *PodSpecStorage
	knownPods   map[string]*corev1.Pod

	// cniNet      string
	// clusterDns  net.IP
}

//cniNet string, clusterDns net.IP
func NewPodManager(podmanClient *podman.PodmanClient, storage *PodSpecStorage) (*PodManager, error) {
	// specStorage, err := NewPodSpecStorage()
	// if err != nil {
	// 	return nil, err
	// }

	storedPodList, err := storage.ListAllPods()
	if err != nil {
		return nil, err
	}
	log.Println("Pods: There are", len(storedPodList), "stored pods")

	foundPods, err := podmanClient.PodPs(context.TODO())
	if err != nil {
		return nil, err
	}
	foundPodMap := make(map[PodCoord]*podman.ListPodsReport)
	for _, foundPod := range foundPods {
		if heritage, ok := foundPod.Labels["heritage"]; ok && heritage == "kube-pet-node" {
			if podCoord, ok := ParsePodKey(foundPod.Name); ok {
				foundPodMap[podCoord] = foundPod
			}
		}
	}
	log.Println("Pods: There are", len(foundPodMap), "found pods")

	knownPods := make(map[string]*corev1.Pod)
	for _, storedPod := range storedPodList {
		foundPod, ok := foundPodMap[storedPod]
		if ok {
			delete(foundPodMap, storedPod)

			podSpec, err := storage.ReadPod(storedPod)
			if err != nil {
				return nil, err
			}
			log.Println("Correlated podspec for", storedPod)
			// TODO: diff?
			// TODO: stored in knownPods
			knownPods[foundPod.Name] = podSpec

		} else {
			log.Println("Pods: Stored pod", storedPod, "wasn't found, deleting from store")
			if err := storage.RemovePod(storedPod); err != nil {
				return nil, err
			}
		}
	}

	for coord, foundPod := range foundPodMap {
		log.Println("Pods: Found dangling pod", coord, "that wasn't stored, deleting from system")
		result, err := podmanClient.PodRm(context.TODO(), foundPod.Id, true)
		if err != nil {
			return nil, err
		}
		log.Println("Pods: Dangling pod rm result:", result)
	}

	eventStream, err := podmanClient.StreamEvents(context.TODO())
	if err != nil {
		return nil, err
	}
	// TODO: plumb these somewhere
	go func() {
		for evt := range eventStream {
			log.Printf("Pods: Event %v %v %v %+v", evt.Type, evt.Action, evt.Status, evt.Actor)
		}
		log.Println("Pods: No more podman events")
	}()

	return &PodManager{
		podman:      podmanClient,
		specStorage: storage,
		knownPods:   knownPods,

		// cniNet:      cniNet,
		// clusterDns: clusterDns,
	}, nil
}

func (pm *PodManager) RuntimeVersionReport(ctx context.Context) (*podman.DockerVersionReport, error) {
	return pm.podman.Version(ctx)
}

func (pm *PodManager) RegisterPod(pod *corev1.Pod) (PodCoord, error) {
	podCoord, err := pm.specStorage.StorePod(pod)
	if err != nil {
		return podCoord, err
	}
	log.Println("Pod", podCoord, "stored")

	// TODO: mutex
	pm.knownPods[podCoord.Key()] = pod
	return podCoord, nil
}

func (pm *PodManager) UnregisterPod(podCoord PodCoord) error {
	// TODO: mutex
	delete(pm.knownPods, podCoord.Key())

	err := pm.specStorage.RemovePod(podCoord)
	if err != nil {
		return err
	}

	log.Println("Pod", podCoord, "removed from store")
	return nil
}

func (pm *PodManager) StartExecInPod(ctx context.Context, podCoord PodCoord, containerName string, options *podman.ContainerExecOptions) (*podman.ExecSession, error) {
	return pm.podman.ContainerExec(ctx, podCoord.ContainerKey(containerName), options)
}
