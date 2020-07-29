package autoupgrade

import (

	"github.com/go-ini/ini"
)

type PlatformCoord struct {
	Os string
	Arch string
	Manager string
}

type TargetRelease struct {
	Version string
	BaseUrl string
	Urls map[PlatformCoord] string
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

	return release, nil


	// states := make(map[string]string)
	// for key, value := range data.KeysHash() {
	// 	if strings.HasSuffix(key, "State") {
	// 		newKey := strings.TrimSuffix(key, "State")
	// 		states[newKey] = value
	// 	}
	// }


	// panic("TODO")
	// return nil, nil
}

/*
    Version = 0.1.2-h44.7ab52ba
    BaseUrl = https://s3-us-west-2.amazonaws.com/dist.stardustapp.run/
    [Target]
    OS = linux
    Arch = amd64
    DebUrl = deb/kube-pet-node_0.1.2-h44.a66d08a_amd64.deb
    RpmUrl = rpm/kube-pet-node-0.1.2-h44.a66d08a.x86_64.rpm
*/
