package pods

import (
	"context"
	"io"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/danopia/kube-pet-node/podman"
)

type PodManager struct {
	podman      *podman.PodmanClient
	specStorage *PodSpecStorage
	KnownPods   map[string]RunningPod

	// cniNet      string
	// clusterDns  net.IP
}

func (pm *PodManager) GetPodman() *podman.PodmanClient {
	return pm.podman
}

type RunningPod struct {
	Kube  *corev1.Pod
	Coord PodCoord
	PodId string
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

	knownPods := make(map[string]RunningPod)
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
			knownPods[foundPod.Name] = RunningPod{podSpec, storedPod, foundPod.Id}

		} else {
			log.Println("Pods: Stored pod", storedPod, "wasn't found, deleting from store")
			if err := storage.RemovePod(storedPod); err != nil {
				return nil, err
			}
		}
	}
	log.Println("Pods: There are", len(knownPods), "known pods")

	for coord, foundPod := range foundPodMap {
		log.Println("Pods: Found dangling pod", coord, "that wasn't stored, deleting from system")
		result, err := podmanClient.PodRm(context.TODO(), foundPod.Id, true)
		if err != nil {
			return nil, err
		}
		log.Println("Pods: Dangling pod rm result:", result)
	}

	// foundVols, err := podmanClient.VolumeList(context.TODO(), map[string][]string{"tag"})

	// starting in podman 2.0.3 / 2.0.4, the events response header isn't flushed until the first event happens
	go func() error {
		eventStream, err := podmanClient.StreamEvents(context.TODO())
		if err != nil {
			return err
		}
		// TODO: plumb these somewhere
		for evt := range eventStream {
			log.Printf("Pods: Event %v %v %v %+v", evt.Type, evt.Action, evt.Status, evt.Actor)
		}
		log.Println("Pods: No more podman events")
		return nil
	}()

	log.Println("Creating PodManager")
	return &PodManager{
		podman:      podmanClient,
		specStorage: storage,
		KnownPods:   knownPods,

		// cniNet:      cniNet,
		// clusterDns: clusterDns,
	}, nil
}

func (pm *PodManager) RuntimeVersionReport(ctx context.Context) (*podman.DockerVersionReport, error) {
	return pm.podman.Version(ctx)
}

func (pm *PodManager) SetPodId(coord PodCoord, podId string) {
	if known, ok := pm.KnownPods[coord.Key()]; ok {
		pm.KnownPods[coord.Key()] = RunningPod{known.Kube, coord, podId}
		log.Println("Pods: Created pod", podId, "for", coord)
	} else {
		log.Println("Pods WARN: SetPodId missed for", coord, "- pod", podId)
	}
}

func (pm *PodManager) RegisterPod(pod *corev1.Pod) (PodCoord, error) {
	podCoord, err := pm.specStorage.StorePod(pod)
	if err != nil {
		return podCoord, err
	}
	log.Println("Pods:", podCoord, "registered")

	// TODO: mutex
	if known, ok := pm.KnownPods[podCoord.Key()]; ok {
		pm.KnownPods[podCoord.Key()] = RunningPod{pod, podCoord, known.PodId}
	} else {
		pm.KnownPods[podCoord.Key()] = RunningPod{pod, podCoord, ""}
	}
	return podCoord, nil
}

func (pm *PodManager) UnregisterPod(podCoord PodCoord) error {
	// TODO: mutex
	delete(pm.KnownPods, podCoord.Key())

	err := pm.specStorage.RemovePod(podCoord)
	if err != nil {
		return err
	}

	log.Println("Pods:", podCoord, "removed from store")
	return nil
}

func (pm *PodManager) GetAllStats(ctx context.Context) (map[*metav1.ObjectMeta][]*podman.PodStatsReport, error) {
	// fetch all container reports
	allReports, err := pm.podman.PodStats(ctx)
	if err != nil {
		return nil, err
	}

	// group the reports by pod ID
	podStats := make(map[string][]*podman.PodStatsReport)
	for _, report := range allReports {
		if others, ok := podStats[report.Pod]; ok {
			podStats[report.Pod] = append(others, report)
		} else {
			podStats[report.Pod] = []*podman.PodStatsReport{report}
		}
	}

	// associate reports with k8s pod metadata
	podMap := make(map[*metav1.ObjectMeta][]*podman.PodStatsReport)
	for _, pod := range pm.KnownPods {
		if pod.PodId == "" {
			log.Println("Pods WARN: lacking ID for pod", pod.Coord)
			continue
		}
		if reports, ok := podStats[pod.PodId[:12]]; ok {
			podMap[&pod.Kube.ObjectMeta] = reports
		} else {
			log.Println("Pods WARN: lacking stats for pod", pod.Coord)
		}
	}

	return podMap, nil
}

func (pm *PodManager) StartExecInPod(ctx context.Context, podCoord PodCoord, containerName string, options *podman.ContainerExecOptions) (*podman.ExecSession, error) {
	return pm.podman.ContainerExec(ctx, podCoord.ContainerKey(containerName), options)
}
func (pm *PodManager) FetchContainerLogs(ctx context.Context, podCoord PodCoord, containerName string, options *podman.ContainerLogsOptions) (io.ReadCloser, error) {
	return pm.podman.ContainerLogs(ctx, podCoord.ContainerKey(containerName), options)
}
