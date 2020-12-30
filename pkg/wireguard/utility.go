package wireguard

import "os/exec"

func GetInstalledVersion() (string, error) {
	out, err := exec.Command("wg", "version").Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}
