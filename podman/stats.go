package podman

import (
	"strconv"
	"strings"

	units "github.com/docker/go-units"
)

type PodStatsReport struct {
	CPU      string // 9.45%
	MemUsage string // 132.8MB / 33.55GB
	Mem      string // 0.40%
	NetIO    string // 61.81kB / 206.8kB
	BlockIO  string // 6.423MB / 1.62MB
	PIDS     string // 15
	Pod      string // 028e5e6fde5e
	CID      string // 7299bd337ee4
	Name     string // media_radarr-srv-0_srv
}

func (psr PodStatsReport) ReadCpuPercent() (float64, error) {
	return readPercentString(psr.CPU)
}

func (psr PodStatsReport) ReadMemPerecent() (float64, error) {
	return readPercentString(psr.Mem)
}

func readPercentString(encoded string) (float64, error) {
	if encoded[0] == '-' {
		return 0.0, nil
	}

	f, err := strconv.ParseFloat(strings.TrimSuffix(encoded, "%"), 64)
	return f * 100, err
}

func (psr PodStatsReport) ReadMemUsage() (uint64, uint64, error) {
	return splitHumanValues(psr.MemUsage)
}

func (psr PodStatsReport) ReadNetIO() (uint64, uint64, error) {
	return splitHumanValues(psr.NetIO)
}

func (psr PodStatsReport) ReadBlockIO() (uint64, uint64, error) {
	return splitHumanValues(psr.BlockIO)
}

func splitHumanValues(combined string) (uint64, uint64, error) {
	if combined[0] == '-' {
		return 0, 0, nil
	}

	parts := strings.Split(combined, " / ")

	left, err := units.FromHumanSize(parts[0])
	if err != nil {
		return 0, 0, err
	}

	right, err := units.FromHumanSize(parts[1])
	if err != nil {
		return 0, 0, err
	}

	return uint64(left), uint64(right), nil
}

func (psr PodStatsReport) ReadPids() (uint64, error) {
	if psr.PIDS[0] == '-' {
		return 0, nil
	}

	return strconv.ParseUint(psr.PIDS, 10, 32)
}
