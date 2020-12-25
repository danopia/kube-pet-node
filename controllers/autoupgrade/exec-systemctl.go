package autoupgrade

import (
	"log"
	"os/exec"
	"strings"

	ini "gopkg.in/ini.v1"
)

func IsUnitRunning(unitName string) (isRunning bool, err error) {
	states, err := GetUnitStates(unitName)
	if err != nil {
		return false, err
	}

	if states["Load"] != "loaded" {
		log.Printf("AutoUpgrade: Unit %s LoadState was %s instead of %s", unitName, states["Load"], "loaded")
		return false, nil
	}
	// It's valid to run the service without it being enabled
	// if states["UnitFile"] != "enabled" {
	// 	log.Printf("AutoUpgrade: Unit %s UnitFileState was %s instead of %s", unitName, states["UnitFile"], "enabled")
	// 	return false, nil
	// }
	if states["Sub"] != "running" {
		log.Printf("AutoUpgrade: Unit %s SubState was %s instead of %s", unitName, states["Sub"], "running")
		return false, nil
	}

	return true, nil
}

func GetUnitStates(unitName string) (map[string]string, error) {
	out, err := exec.Command("systemctl", "show", "--", unitName).Output()
	if err != nil {
		return nil, err
	}

	file, err := ini.Load(out)
	if err != nil {
		return nil, err
	}

	data := file.Section("DEFAULT")

	states := make(map[string]string)
	for key, value := range data.KeysHash() {
		if strings.HasSuffix(key, "State") {
			newKey := strings.TrimSuffix(key, "State")
			states[newKey] = value
		}
	}

	return states, nil
}
