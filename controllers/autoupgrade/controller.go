package autoupgrade

import (
	"context"
	"log"
	"time"
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
	IsCapable bool
	SelfVersion *semver.Version
	// ConfigMapInformer corev1informers.ConfigMapInformer

	releaseChan chan *TargetRelease
}

func NewAutoUpgrade(cmi corev1informers.ConfigMapInformer) (*AutoUpgrade, error) {

	installedVersion, err := GetInstalledVersion("kube-pet-node")
	if err != nil {
		log.Println("AutoUpgrade WARN: failed to find installed version:", err)
		return &AutoUpgrade{}, nil
	}

	parsedVersion, err := semver.NewVersion(installedVersion)
	if err != nil {
		return nil, err
	}

	if isRunning, err := IsUnitRunning("kube-pet-node.service"); err != nil {
		return nil, err
	} else if !isRunning {
		log.Println("AutoUpgrade WARN: our systemd unit is not running")
		return &AutoUpgrade{}, nil
	}

	controller := &AutoUpgrade{
		IsCapable: true,
		SelfVersion: parsedVersion,
		// ConfigMapInformer: cmi,

		releaseChan: make(chan *TargetRelease, 0),
	}

	cmi.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(res interface{}) {
			if res, ok := res.(*corev1.ConfigMap); ok {
				controller.enqueueConfigMap(res)
			}
		},
		UpdateFunc: func(before, after interface{}) {
			if res, ok := after.(*corev1.ConfigMap); ok {
				controller.enqueueConfigMap(res)
			}
		},
		DeleteFunc: func(res interface{}) {
			if res, ok := res.(*corev1.ConfigMap); ok {
				if res.ObjectMeta.Namespace == "kube-system" && res.ObjectMeta.Name == "kube-pet-node" {
					controller.releaseChan <- nil
				}
			}
		},
	})
	log.Println("AutoUpgrade: Configured TargetRelease informer")

	return controller, nil
}

func (ka *AutoUpgrade) enqueueConfigMap(res *corev1.ConfigMap) {
	if res.ObjectMeta.Namespace == "kube-system" && res.ObjectMeta.Name == "kube-pet-node" {
		if releaseStr, ok := res.Data["TargetRelease"]; ok {
			if releaseInfo, err := ParseTargetRelease(releaseStr); err != nil {
				log.Println("AutoUpgrade WARN: Failed to read ConfigMap:", err)
			} else {
				// Pass down the parsed info
				ka.releaseChan <- releaseInfo
				return
			}
		}
		// If we couldn't find release info, still inform downstream
		ka.releaseChan <- nil
	}
}

func (ka *AutoUpgrade) Run(ctx context.Context) {

	if !ka.IsCapable {
		log.Println("AutoUpgrade: Leaving disabled for the lifetime of this process.")
		return
	}

	var timerC <-chan time.Time
	var targetRelease *TargetRelease

	for {
		select {
		case targetRelease = <-ka.releaseChan:
			// Cancel whatever timer we had anytime we get an update
			timerC = nil

			if targetRelease == nil {
				log.Println("AutoUpgrade: received empty release info")
				continue
			}
			if !targetRelease.AutoUpgrade {
				log.Println("AutoUpgrade: received disabled release info")
				continue
			}

			targetVersion, err := semver.NewVersion(targetRelease.Version)
			if err != nil {
				log.Println("AutoUpgrade WARN: failed to parse received release version", err)
				continue
			}

			if !ka.SelfVersion.LessThan(*targetVersion) {
				log.Println("AutoUpgrade: Our version", ka.SelfVersion, "isn't older than target version", targetVersion)
				continue
			}
			if !targetRelease.HasBuildForUs() {
				log.Println("AutoUpgrade: Target version", targetVersion, "doesn't have a build for us")
				continue
			}

			log.Println("AutoUpgrade: Version", targetVersion, "looks newer than our", ka.SelfVersion, "- scheduling upgrade")

			// TODO: randomize this over like 60 seconds probably
			timerC = time.After(5 * time.Second)

		case <-timerC:
			log.Println("AutoUpgrade: NOW INSTALLING", targetRelease.Version)

			if err := targetRelease.ActuallyInstallThisReleaseNow(); err != nil {
				log.Println("AutoUpgrade: Failed to perform upgrade:", err)
			}
			log.Println("AutoUpgrade: Okay, I tried doing a thing, I'm disabling for the rest of this process lifetime.")
			return
		}
	}
}
