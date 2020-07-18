package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	corev1 "k8s.io/api/core/v1"

	"github.com/adrg/xdg"
)

type PodCoord struct {
	Namespace string
	Name      string
}

func (pc PodCoord) fileName() string {
	return fmt.Sprintf("%v_%v.json", pc.Namespace, pc.Name)
}

type PodSpecStorage struct {
	rootDir string
	lock    sync.RWMutex
}

func NewPodSpecStorage() (*PodSpecStorage, error) {
	rootDir := xdg.CacheHome + "/kube-pet-node/pod-specs"

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

	list := make([]PodCoord, len(files))
	for idx, file := range files {
		log.Println(file.Name())
		list[idx] = PodCoord{"a", "b"}
	}

	log.Println("Stored pods:", list)
	return list, nil
}

func (pss *PodSpecStorage) StorePod(podSpec *corev1.Pod) (*PodCoord, error) {
	coord := &PodCoord{
		Namespace: podSpec.ObjectMeta.Namespace,
		Name:      podSpec.ObjectMeta.Name,
	}
	filePath := pss.rootDir + "/" + coord.fileName()

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
