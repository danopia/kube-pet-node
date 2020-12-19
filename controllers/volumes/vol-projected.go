package volumes

import (
	"context"
	"log"
	"strconv"
	"strings"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ctl *VolumesController) CreateProjectedVolume(ctx context.Context, pod *corev1.Pod, volName string, projectedSource *corev1.ProjectedVolumeSource) error {

	paths := make(map[string][]byte)

	var mode int64 = 0644
	if projectedSource.DefaultMode != nil {
		mode = int64(*projectedSource.DefaultMode)
	}

	for _, source := range projectedSource.Sources {

		if source.ServiceAccountToken != nil {
			saName := "default"
			if pod.Spec.ServiceAccountName != "" {
				saName = pod.Spec.ServiceAccountName
			}

			tokenReq, err := ctl.coreV1.
				ServiceAccounts(pod.ObjectMeta.Namespace).
				CreateToken(ctx, saName, &authv1.TokenRequest{
					Spec: authv1.TokenRequestSpec{
						Audiences:         []string{source.ServiceAccountToken.Audience},
						ExpirationSeconds: source.ServiceAccountToken.ExpirationSeconds,
					},
				}, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			paths[source.ServiceAccountToken.Path] = []byte(tokenReq.Status.Token)
		}
	}

	volPath, err := ctl.CreateRuntimeVolume(ctx, string(pod.ObjectMeta.UID), "projected", volName)
	if err != nil {
		return err
	}

	tar, err := startArchiveExtractin(ctx, volPath)
	if err != nil {
		return err
	}

	resVersion := strconv.FormatInt(tar.now.Unix(), 10)
	for path, data := range paths {
		tar.WriteFile("..data/"+resVersion+"/"+path, mode, data)
	}
	tar.WriteSymLink("..data/current", resVersion)

	// TODO: only emit these on first setup
	for path := range paths {
		tar.WriteSymLink(path, strings.Repeat("../", strings.Count(path, "/"))+"..data/current/"+path)
	}

	if err := tar.Finish(); err != nil {
		return err
	}

	log.Println("Volumes: Made projected vol for", volName, "at", volPath)
	return nil
}

// $ echo '{"spec":{"audiences":["asdf"]}}'|kubectl create --raw /api/v1/namespaces/kube-pets/serviceaccounts/default/token -f - --kubeconfig node-kubeconfig.yaml
// {
//   "kind":"TokenRequest","apiVersion":"authentication.k8s.io/v1",
//   "metadata":{"selfLink":"/api/v1/namespaces/kube-pets/serviceaccounts/default/token","creationTimestamp":null},
//   "spec":{"audiences":["asdf"],"expirationSeconds":3600,"boundObjectRef":null},
//   "status":{"token":"..............","expirationTimestamp":"2020-10-26T08:46:30Z"}
// }
