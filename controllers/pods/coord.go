package pods

import (
	"fmt"
	"strings"
)

type PodCoord struct {
	Namespace string
	Name      string
}

func ParsePodKey(key string) (coord PodCoord, ok bool) {
	parts := strings.Split(key, "_")
	if len(parts) == 2 {
		coord.Namespace = parts[0]
		coord.Name = parts[1]
		ok = coord.Namespace != "" && coord.Name != ""
	}
	return
}

func (pc PodCoord) FileName() string {
	return fmt.Sprintf("%v_%v.json", pc.Namespace, pc.Name)
}

func (pc PodCoord) Key() string {
	return fmt.Sprintf("%v_%v", pc.Namespace, pc.Name)
}
