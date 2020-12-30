package volumes

import (
	"context"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/danopia/kube-pet-node/pkg/fsinject"
)

func (ctl *VolumesController) CreateSecretVolume(ctx context.Context, podMeta *metav1.ObjectMeta, volName string, secretSource *corev1.SecretVolumeSource) error {

	volPath, err := ctl.CreateRuntimeVolume(ctx, string(podMeta.UID), "secret", volName)
	if err != nil {
		return err
	}

	isOpt := secretSource.Optional != nil && *secretSource.Optional == true

	secret, err := ctl.caching.GetSecret(ctx, podMeta.Namespace, secretSource.SecretName)
	if err != nil {
		log.Println("Volumes: secret err", err)
		if isOpt {
			log.Println("Volumes: Optional secret", podMeta.Namespace, secretSource.SecretName, "gone:", err, "- ignoring")
			return nil
		}
		return err
	}
	resVersion := secret.ObjectMeta.ResourceVersion

	tar, err := fsinject.StartArchiveExtraction(ctx, volPath)
	if err != nil {
		return err
	}

	var mode int64 = 0644
	if secretSource.DefaultMode != nil {
		mode = int64(*secretSource.DefaultMode)
	}

	if secretSource.Items == nil {

		for key, data := range secret.Data {
			tar.WriteFile("..data/"+resVersion+"/"+key, mode, data)
		}
		tar.WriteSymLink("..data/current", resVersion)

		// TODO: only emit these on first setup
		for key := range secret.Data {
			tar.WriteSymLink(key, "..data/current/"+key)
		}

	} else {

		for _, item := range secretSource.Items {
			var itemMode int64 = mode
			if item.Mode != nil {
				itemMode = int64(*item.Mode)
			}
			if data, ok := secret.Data[item.Key]; ok {
				tar.WriteFile("..data/"+resVersion+"/"+item.Path, itemMode, data)
			} else if isOpt {
				log.Println("Volumes: Optional secret key", item.Key, "missing")
			} else {
				log.Println("Volumes TODO: missing key!")
			}
		}
		tar.WriteSymLink("..data/current", resVersion)

		// TODO: only emit these on first setup
		for _, item := range secretSource.Items {
			tar.WriteSymLink(item.Path, strings.Repeat("../", strings.Count(item.Path, "/"))+"..data/current/"+item.Path)
		}

	}

	if err := tar.Finish(); err != nil {
		return err
	}

	log.Println("Volumes: Made secret vol for", volName, "at", volPath)
	return nil
}
