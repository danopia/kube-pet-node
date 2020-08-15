package pods

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

type PodSpecStorage struct {
	rootDir string
	lock    sync.RWMutex
}

func NewPodSpecStorage() (*PodSpecStorage, error) {
	cacheHome, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Join(cacheHome, "kube-pet-node", "pod-specs")

	if err := os.MkdirAll(rootDir, 0700); err != nil {
		return nil, err
	}

	return &PodSpecStorage{
		rootDir: rootDir,
	}, nil
}

func (pss *PodSpecStorage) ListAllPods() ([]PodCoord, error) {
	pss.lock.RLock()
	defer pss.lock.RUnlock()

	f, err := os.Open(pss.rootDir)
	if err != nil {
		return nil, err
	}

	files, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	list := make([]PodCoord, 0, len(files))
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			podKey := strings.TrimSuffix(file.Name(), ".json")
			if podCoord, ok := ParsePodKey(podKey); ok {
				list = append(list, podCoord)
			}
		}
	}

	// log.Println("Pods: Found stored pods", list)
	return list, nil
}

func (pss *PodSpecStorage) StorePod(podSpec *corev1.Pod) (PodCoord, error) {
	coord := PodCoord{
		Namespace: podSpec.ObjectMeta.Namespace,
		Name:      podSpec.ObjectMeta.Name,
	}
	filePath := filepath.Join(pss.rootDir, coord.FileName())

	pss.lock.Lock()
	defer pss.lock.Unlock()

	file, err := os.Create(filePath)
	if err != nil {
		return coord, err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	if err := enc.Encode(podSpec); err != nil {
		return coord, err
	}

	return coord, nil
}

func (pss *PodSpecStorage) ReadPod(coord PodCoord) (*corev1.Pod, error) {
	filePath := filepath.Join(pss.rootDir, coord.FileName())

	pss.lock.RLock()
	defer pss.lock.RUnlock()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var podSpec corev1.Pod
	dec := json.NewDecoder(file)
	if err := dec.Decode(&podSpec); err != nil {
		return nil, err
	}

	return &podSpec, nil
}

func (pss *PodSpecStorage) RemovePod(coord PodCoord) error {
	filePath := filepath.Join(pss.rootDir, coord.FileName())

	pss.lock.Lock()
	defer pss.lock.Unlock()

	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
