package autoupgrade

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var hasDpkg bool
var hasDnf bool

var dnfVerPattern *regexp.Regexp = regexp.MustCompile("(?m)^Version +: (.+)$")
var dnfRelPattern *regexp.Regexp = regexp.MustCompile("(?m)^Release +: (.+)$")

func init() {
	if err := exec.Command("/usr/bin/env", "which", "dpkg-query").Run(); err == nil {
		hasDpkg = true
	}
	if err := exec.Command("/usr/bin/env", "which", "dnf").Run(); err == nil {
		hasDnf = true
	}
}

func SystemType() string {
	if hasDpkg && hasDnf {
		panic("TODO: System has both dpkg and dnf present")
	} else if hasDpkg {
		return "Deb"
	} else if hasDnf {
		return "Rpm"
	} else {
		panic("TODO: System has neither dpkg nor dnf present")
	}
}

func GetInstalledVersion(pkgName string) (string, error) {
	switch SystemType() {

	case "Deb":
		// dpkg keeps pretty detailed status flags so we check them here
		out, err := exec.Command("dpkg-query", "-W", "--showformat=${Version} ${Status}", "--", pkgName).Output()
		if err != nil {
			return "", err
		}

		// [version, 'install' or 'hold' or 'deinstall', 'ok', 'installed' or 'config-files' or 'half-configured']
		parts := strings.Split(string(out), " ")
		if len(parts) != 4 {
			return "", fmt.Errorf("dpkg-query gave %v fields instead of 4", len(parts))
		}

		if parts[1] != "install" {
			return "", fmt.Errorf("dpkg-query reported our package as '%s' which wasn't 'install'", parts[1])
		}
		if parts[2] != "ok" {
			return "", fmt.Errorf("dpkg-query reported our package as '%s' which wasn't 'ok'", parts[2])
		}
		if parts[3] != "installed" && parts[3] != "half-configured" {
			return "", fmt.Errorf("dpkg-query reported our package as '%s' which wasn't 'installed' or 'half-configured'", parts[3])
		}
		return parts[0], nil

	case "Rpm":
		// seems to really just be the metadata so we just grab version if it's installed at all
		out, err := exec.Command("dnf", "info", "--installed", "--", pkgName).Output()
		if err != nil {
			return "", err
		}

		var version string = "missing"
		var release string = "missing"
		if match := dnfVerPattern.FindSubmatch(out); match != nil {
			version = string(match[1])
		} else {
			return "", fmt.Errorf("dnf version pattern didn't match")
		}
		if match := dnfRelPattern.FindSubmatch(out); match != nil {
			release = string(match[1])
		} else {
			return "", fmt.Errorf("dnf release pattern didn't match")
		}
		return fmt.Sprintf("%s-%s", version, release), nil

	default:
		panic("TODO: Unhandled system package manager")
	}
}
