package pods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/danopia/kube-pet-node/pkg/podman"
)

func (d *PodmanProvider) GrabPullSecrets(ctx context.Context, namespace string, secretRefs []corev1.LocalObjectReference) ([]*corev1.Secret, error) {
	secrets := make([]*corev1.Secret, len(secretRefs))
	for idx, ref := range secretRefs {
		secret, err := d.caching.GetSecret(ctx, namespace, ref.Name)
		if err != nil {
			return secrets, err
		}
		secrets[idx] = secret
	}
	return secrets, nil
}

func processPullStream(events <-chan podman.ImagePullReport) (ids []string, err error) {
	var lastLine string
	for event := range events {
		switch {
		case event.Stream != "":
			log.Println("PULL STDOUT:", event.Stream)
			lastLine = event.Stream
		case event.Error != "":
			log.Println("PULL STDERR:", event.Error)
			err = errors.New(event.Error)
		case len(event.Images) > 0:
			ids = event.Images
		}
	}
	// check for cases where there's no error and no IDs
	if len(ids) == 0 && err == nil {
		log.Println("PULL RESULT MISSING!")
		if lastLine != "" {
			err = errors.New(lastLine)
		}
	}
	return
}

func (d *PodmanProvider) PullImage(ctx context.Context, imageRef string, pullSecrets []*corev1.Secret, podRef *corev1.ObjectReference) error {
	d.events.Eventf(podRef, corev1.EventTypeNormal, "Pulling", "Pulling image \"%s\"", imageRef)

	pullStream, err := d.podman.Pull(ctx, imageRef, nil)
	if err == nil {
		imgIds, err := processPullStream(pullStream)
		if err == nil {
			log.Println("Pulled images", imgIds, "for", podRef.Namespace, "/", podRef.Name)
			d.events.Eventf(podRef, corev1.EventTypeNormal, "Pulled", "Successfully pulled public image \"%s\"", imageRef)
			return nil
		} else if !strings.Contains(err.Error(), "unauthorized") {
			return err
		}
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
			return fmt.Errorf("Failed to read dockerconfigjson from Secret: %w", err)
		}
		authBody, err := json.Marshal(&dockerconf.Auths)
		if err != nil {
			return fmt.Errorf("Failed to encode docker auths from secret: %w", err)
		}

		pullStream, err = d.podman.Pull(ctx, imageRef, authBody)
		if err == nil {
			imgIds, err := processPullStream(pullStream)
			if err == nil {
				log.Println("Pulled PRIVATE images", imgIds, "for", podRef.Namespace, "/", podRef.Name)
				d.events.Eventf(podRef, corev1.EventTypeNormal, "Pulled", "Successfully pulled private image \"%s\"", imageRef)
				return nil
			} else if !strings.Contains(err.Error(), "unauthorized") {
				return err
			}
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
