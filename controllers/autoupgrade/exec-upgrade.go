package autoupgrade

import (
	"fmt"
	"os/exec"
)

func (tr *TargetRelease) ActuallyInstallThisReleaseNow() error {

	buildUrl := tr.GetOurBuildUrl()
	if buildUrl == "" {
		return fmt.Errorf("Can't install release %v because it lacks a URL for us", tr.Version)
	}

	cmd := exec.Command("sudo", "/opt/kube-pet-node/bin/node-upgrade.sh", tr.Version, SystemType(), buildUrl)
	return cmd.Run()
}
