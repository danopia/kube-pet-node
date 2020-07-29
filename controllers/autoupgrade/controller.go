package autoupgrade

import (
	"context"
	"log"
	// "time"
	// "net"
	// "net/http"
	// "os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	corev1informers "k8s.io/client-go/informers/core/v1"

	"github.com/coreos/go-semver/semver"
	// "github.com/bep/debounce"
)

type AutoUpgrade struct {
	IsActive bool
	SelfVersion *semver.Version
	TargetVersion *semver.Version
	ConfigMapInformer corev1informers.ConfigMapInformer
}

func NewAutoUpgrade(cmi corev1informers.ConfigMapInformer) (*AutoUpgrade, error) {

	installedVersion, err := GetInstalledVersion("kube-pet-node")
	if err != nil {
		log.Println("AutoUpgrade: WARN: failed to find version, disabling.", err)
		return &AutoUpgrade{}, nil
	}

	parsedVersion, err := semver.NewVersion(installedVersion)
	if err != nil {
		return nil, err
	}

	isRunning, err := IsUnitRunning("kube-pet-node.service")
	if err != nil {
		return nil, err
	}

	return &AutoUpgrade{
		IsActive: isRunning,
		SelfVersion: parsedVersion,
		ConfigMapInformer: cmi,
	}, nil
}

func (ka *AutoUpgrade) Run(ctx context.Context) {



	log.Println("AutoUpgrade: Setting up informer")
	// ka.ConfigMapInformer.Informer().AddEventHandler(func(){})
	ka.ConfigMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(res interface{}) {
			if res, ok := res.(*corev1.ConfigMap); ok {
				if res.ObjectMeta.Namespace == "kube-system" && res.ObjectMeta.Name == "kube-pet-node" {
					if releaseInfo, err := ParseTargetRelease(res.Data["TargetRelease"]); err != nil {
						log.Println("AutoUpgrade WARN: Failed to read ConfigMap:", err)
					} else {
						log.Fatalf("AutoUpgrade got target:", releaseInfo)
					}
				}
			}
		},
		DeleteFunc: func(interface{}) {
			// ka.Debounce(ka.Sync)
		},
		UpdateFunc: func(before, after interface{}) {
			// panic("TODO")
			// Ignore notificatins to control-plane election endpoints.
			//  "[T]here are plans for changing the leader election mechanism based on endpoints
			//   in favour of a similar approach based on config maps.
			//   This avoids continuously triggering “endpoint-changed” notifications
			//   to kube-proxy and kube-dns"
			if before, ok := before.(*corev1.Endpoints); ok {
				if before.ObjectMeta.Namespace == "kube-system" {
					if _, ok := before.ObjectMeta.Annotations["control-plane.alpha.kubernetes.io/leader"]; ok {
						return
					}
				}
				// log.Println("Updated:", before.ObjectMeta.Namespace, before.ObjectMeta.Name)
			}

			// ka.Debounce(ka.Sync)
		},
	})

	// log.Println("AutoUpgrade: Seeding debounce")
	// ka.Debounce(ka.Sync)
}
