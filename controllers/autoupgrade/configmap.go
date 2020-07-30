package autoupgrade

import (
	"runtime"

	"github.com/go-ini/ini"
)

type TargetRelease struct {
	Version string
	BaseUrl string
	AutoUpgrade bool
	Platforms []TargetPlatform
}
type TargetPlatform struct {
	OS string
	Arch string
	DebUrl string
	RpmUrl string
}

func ParseTargetRelease(data string) (*TargetRelease, error) {

	file, err := ini.Load([]byte(data))
	if err != nil {
		return nil, err
	}

	release := &TargetRelease{}

	if err := file.Section("DEFAULT").StrictMapTo(release); err != nil {
		return nil, err
	}

	platformSects, err := file.SectionsByName("Platform")
	if err != nil {
		return nil, err
	}

	release.Platforms = make([]TargetPlatform, len(platformSects))
	for idx, section := range platformSects {
		if err := section.StrictMapTo(&release.Platforms[idx]); err != nil {
			return nil, err
		}
	}

	return release, nil
}

func (tr *TargetRelease) HasBuildForUs() bool {
	return tr.GetOurBuildUrl() != ""
}

func (tr *TargetRelease) GetOurBuildUrl() string {
	for _, plat := range tr.Platforms {
		if plat.OS == runtime.GOOS && plat.Arch == runtime.GOARCH {
			switch SystemType() {
				case "Deb":
					if plat.DebUrl != "" {
						return tr.BaseUrl+plat.DebUrl
					}
				case "Rpm":
					if plat.RpmUrl != "" {
						return tr.BaseUrl+plat.RpmUrl
					}
				default: return ""
			}
		}
	}
	return ""
}
