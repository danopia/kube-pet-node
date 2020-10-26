package volumes

import (
	"context"
	"log"

	"github.com/danopia/kube-pet-node/podman"
)

func (ctl *VolumesController) ListRuntimeVolumes(ctx context.Context, podUid string) ([]string, error) {

	myVolFilter := make(map[string][]string)
	myVolFilter["label"] = []string{"poduid=" + podUid}
	existingVols, err := ctl.podman.VolumeList(ctx, myVolFilter)

	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, vol := range existingVols {
		names = append(names, vol.Name[len(podUid)+1:])
	}

	return names, nil
}

func (ctl *VolumesController) CreateRuntimeVolume(ctx context.Context, podUid string, volType string, volName string) (string, error) {

	myVolLabels := make(map[string]string)
	myVolLabels["poduid"] = podUid
	myVolLabels["voltype"] = volType

	log.Println("Volumes: Making vol for", podUid, volName)
	volInfo, err := ctl.podman.VolumeCreate(ctx, &podman.VolumeCreateOptions{
		Name:  podUid + "_" + volName,
		Label: myVolLabels,
	})
	if err != nil {
		return "", err
	}

	return volInfo.Mountpoint, nil
}

func (ctl *VolumesController) DeleteRuntimeVolume(ctx context.Context, podUid string, volName string, force bool) error {
	log.Println("Volumes: Deleting vol for", podUid, volName)
	return ctl.podman.VolumeRm(ctx, podUid+"_"+volName, force)
}
