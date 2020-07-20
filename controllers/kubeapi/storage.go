package kubeapi

import (
	// "encoding/json"
	// "log"
	"os"
	"path/filepath"
	"sync"
	// "strings"
	// corev1 "k8s.io/api/core/v1"
)

type KeyMaterialStorage struct {
	materialId string
	rootDir    string
	lock       sync.RWMutex
}

func NewKeyMaterialStorage(materialId string) (*KeyMaterialStorage, error) {
	cacheHome, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Join(cacheHome, "kube-pet-node", "key-material")

	if err := os.MkdirAll(rootDir, 0700); err != nil {
		return nil, err
	}

	return &KeyMaterialStorage{
		rootDir:    rootDir,
		materialId: materialId,
	}, nil
}

func (pss *KeyMaterialStorage) GetFilePath(extension string) string {
	return filepath.Join(pss.rootDir, pss.materialId+extension)
}

func (pss *KeyMaterialStorage) EnsurePrivateKeyExists(keygen func(outPath string) error) error {
	keyPath := pss.GetFilePath(".key")

	pss.lock.Lock()
	defer pss.lock.Unlock()

	if _, err := os.Stat(keyPath); err == nil {
		return nil
	}

	// responsible for putting the new key on disk
	return keygen(keyPath)
}

func (pss *KeyMaterialStorage) StoreFile(extension string, data []byte) error {
	filePath := pss.GetFilePath(extension)

	pss.lock.Lock()
	defer pss.lock.Unlock()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return err
	}

	return nil
}

// func (pss *KeyMaterialStorage) ReadPod(coord PodCoord) (*corev1.Pod, error) {
// 	filePath := filepath.Join(pss.rootDir, coord.FileName())

// 	pss.lock.RLock()
// 	defer pss.lock.RUnlock()

// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer file.Close()

// 	var podSpec corev1.Pod
// 	dec := json.NewDecoder(file)
// 	if err := dec.Decode(&podSpec); err != nil {
// 		return nil, err
// 	}

// 	return &podSpec, nil
// }

// func (pss *KeyMaterialStorage) RemovePod(coord PodCoord) error {
// 	filePath := filepath.Join(pss.rootDir, coord.FileName())

// 	pss.lock.Lock()
// 	defer pss.lock.Unlock()

// 	return os.Remove(filePath)
// }
