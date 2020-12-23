package volumes

import (
	// "bytes"
	"context"
	// "hash"
	// "hash/fnv"
	"log"
	// "net"
	// "strings"
	// "time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetes "k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/danopia/kube-pet-node/controllers/caching"
	"github.com/danopia/kube-pet-node/podman"
)

type VolumesController struct {
	coreV1  corev1client.CoreV1Interface
	caching *caching.Controller
	podman  *podman.PodmanClient
}

func NewVolumesController(kubernetes *kubernetes.Clientset, caching *caching.Controller, podman *podman.PodmanClient) *VolumesController {

	log.Println("Volumes: constructing controller")

	return &VolumesController{
		coreV1:  kubernetes.CoreV1(),
		caching: caching,
		podman:  podman,
	}
}

func (ctl *VolumesController) CleanupVolumes(ctx context.Context, podMeta *metav1.ObjectMeta) error {
	podUid := string(podMeta.UID)

	existingVols, err := ctl.ListRuntimeVolumes(ctx, podUid)
	if err != nil {
		return err
	}
	log.Println("Volumes: existing vols:", existingVols)
	for _, volName := range existingVols {
		if err := ctl.DeleteRuntimeVolume(ctx, podUid, volName, false); err != nil {
			log.Println("Volumes: existing vol cleanup err", podUid, volName, err)
			return err
		}
	}

	return nil
}

func (ctl *VolumesController) CreatePodVolumes(ctx context.Context, pod *corev1.Pod) error {
	err := ctl.CleanupVolumes(ctx, &pod.ObjectMeta)
	if err != nil {
		return err
	}

	for _, spec := range pod.Spec.Volumes {

		if spec.VolumeSource.HostPath != nil {
			// nothing, is a bind later
			continue

		} else if spec.VolumeSource.EmptyDir != nil {
			if spec.VolumeSource.EmptyDir.Medium == corev1.StorageMediumMemory {
				// nothing, will make a tmpfs within the container later
				continue
			}
			volPath, err := ctl.CreateRuntimeVolume(ctx, string(pod.ObjectMeta.UID), "emptydir", spec.Name)
			if err != nil {
				return err
			}

			log.Println("Volumes: Made emptydir vol at", volPath)

		} else if spec.VolumeSource.Secret != nil {
			return ctl.CreateSecretVolume(ctx, &pod.ObjectMeta, spec.Name, spec.VolumeSource.Secret)

		} else if spec.VolumeSource.Projected != nil {
			return ctl.CreateProjectedVolume(ctx, pod, spec.Name, spec.VolumeSource.Projected)

		}

		log.Println("Volumes TODO: Volume spec", spec.Name, "isn't supported!")

	}

	return nil
}

// func CreateTarball([]) error {
// 	tarWriter := tar.NewWriter(gzipWriter)
// 	defer tarWriter.Close()

// 	header := &tar.Header{
// 		// Typeflag: tar.TypeSymlink,
// 		// Linkname: "..data",
// 		Name:    "asdf",
// 		Size:    4,
// 		Mode:    int64(0644),
// 		ModTime: stat.ModTime(),
// 	}

// 	err = tarWriter.WriteHeader(header)
// 	if err != nil {
// 		return errors.New(fmt.Sprintf("Could not write header for file '%s', got error '%s'", filePath, err.Error()))
// 	}

// 	_, err = io.Copy(tarWriter, file)
// 	if err != nil {
// 		return errors.New(fmt.Sprintf("Could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error()))
// 	}

// 	return nil
// }
