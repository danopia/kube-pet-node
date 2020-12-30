package kubeapi

import (
	"context"
	"log"
	"strings"
	"time"

	statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	// utilexec "k8s.io/utils/exec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	linuxproc "github.com/c9s/goprocinfo/linux"
	sysconf "github.com/tklauser/go-sysconf"
)

func (ka *KubeApi) GetStatsSummary(ctx context.Context) (*statsv1.Summary, error) {

	// the actual API call part
	nowStamp := metav1.NewTime(time.Now())
	podReports, err := ka.podManager.GetAllStats(ctx)
	if err != nil {
		return nil, err
	}

	// generally, the total number of containers we are running will be quite stable
	newStats := make(map[string]prevStat, len(ka.prevStats))

	podStats := make([]statsv1.PodStats, 0, len(podReports))
	for podMeta, reports := range podReports {
		if len(reports) < 1 {
			continue
		}

		var totalNanoCores uint64
		var totalCPUTime uint64
		var totalMem uint64

		var netStats *statsv1.NetworkStats

		conStats := make([]statsv1.ContainerStats, 0, len(reports)-1)
		for conName, report := range reports {
			reportTime := metav1.NewTime(time.Unix(0, int64(report.SystemNano)))

			newStats[report.ContainerID] = prevStat{
				time: report.SystemNano, cpu: report.CPUNano,
			}
			prevStat, hasPrevStat := ka.prevStats[report.ContainerID]

			memUsed := report.MemUsage
			memAvail := (report.MemLimit - report.MemUsage)

			cpuFrac := 0.0
			if hasPrevStat {
				cpuFrac = calculateCPUFraction(report.CPUNano, prevStat.cpu, report.SystemNano, prevStat.time)
			}
			cpuNano := uint64(cpuFrac * 1000 * 1000 * 1000)
			cpuCumulative := report.CPUNano

			totalNanoCores += cpuNano
			totalCPUTime += report.CPUNano
			totalMem += memUsed

			// Grab any network stats from infra
			if conName == "_infra" && (report.NetInput > 0 || report.NetOutput > 0) {
				netInput := report.NetInput
				netOutput := report.NetOutput
				ifaceStats := statsv1.InterfaceStats{
					Name:    "default",
					RxBytes: &netInput,
					TxBytes: &netOutput,
				}
				netStats = &statsv1.NetworkStats{
					Time:           reportTime,
					InterfaceStats: ifaceStats,
					Interfaces:     []statsv1.InterfaceStats{ifaceStats},
				}
			}

			// Don't return infra containers themselves
			if conName != "_infra" {
				conStats = append(conStats, statsv1.ContainerStats{
					Name: conName,

					StartTime: reportTime,
					CPU: &statsv1.CPUStats{
						Time:                 reportTime,
						UsageNanoCores:       &cpuNano,
						UsageCoreNanoSeconds: &cpuCumulative,
					},
					Memory: &statsv1.MemoryStats{
						Time:            reportTime,
						AvailableBytes:  &memAvail,
						UsageBytes:      &memUsed,
						WorkingSetBytes: &memUsed,
						// RSSBytes:        &memUsed,
					},
					UserDefinedMetrics: []statsv1.UserDefinedMetric{{
						Time: reportTime,
						UserDefinedMetricDescriptor: statsv1.UserDefinedMetricDescriptor{
							Name:  "podman.pids",
							Type:  statsv1.MetricGauge,
							Units: "count",
						},
						Value: float64(report.PIDs),
					}, {
						Time: reportTime,
						UserDefinedMetricDescriptor: statsv1.UserDefinedMetricDescriptor{
							Name:  "podman.cpu_system",
							Type:  statsv1.MetricCumulative,
							Units: "nanoseconds",
						},
						Value: float64(report.CPUSystemNano),
					}, {
						Time: reportTime,
						UserDefinedMetricDescriptor: statsv1.UserDefinedMetricDescriptor{
							Name:  "podman.block_input",
							Type:  statsv1.MetricCumulative,
							Units: "bytes",
						},
						Value: float64(report.BlockInput),
					}, {
						Time: reportTime,
						UserDefinedMetricDescriptor: statsv1.UserDefinedMetricDescriptor{
							Name:  "podman.block_output",
							Type:  statsv1.MetricCumulative,
							Units: "bytes",
						},
						Value: float64(report.BlockOutput),
					}},
				})
			}
		}

		podStats = append(podStats, statsv1.PodStats{
			PodRef: statsv1.PodReference{
				Name:      podMeta.Name,
				Namespace: podMeta.Namespace,
				UID:       string(podMeta.UID),
			},
			StartTime:  nowStamp,
			Containers: conStats,
			CPU: &statsv1.CPUStats{
				Time:                 nowStamp,
				UsageNanoCores:       &totalNanoCores,
				UsageCoreNanoSeconds: &totalCPUTime,
			},
			Memory: &statsv1.MemoryStats{
				Time: nowStamp,
				// AvailableBytes:  &zero,
				UsageBytes:      &totalMem,
				WorkingSetBytes: &totalMem,
				// RSSBytes:        &totalMem,
			},
			Network: netStats,
			// EphemeralStorage: &statsv1.FsStats{
			// 	Time:           nowStamp,
			// 	AvailableBytes: &zero,
			// 	CapacityBytes:  &zero,
			// 	UsedBytes:      &zero,
			// },
			// ProcessStats: &statsv1.ProcessStats{
			// 	ProcessCOunt: &zero,
			// },
		})
	}

	ka.prevStats = newStats

	nodeStats := statsv1.NodeStats{
		NodeName:  ka.nodeName,
		StartTime: nowStamp,
	}

	if stat, err := linuxproc.ReadStat("/proc/stat"); err != nil {
		log.Println("KubeAPI WARN: Failed to read /proc/stat:", err.Error())
	} else {
		clktck, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
		if err != nil {
			clktck = 100
		}

		var cpuTime uint64 = stat.CPUStatAll.User + stat.CPUStatAll.Nice + stat.CPUStatAll.System
		cpuCumulative := cpuTime * 1000 * 1000 * 1000 / uint64(clktck)

		nowNano := uint64(time.Now().UnixNano())

		cpuFrac := 0.0
		if ka.prevNodeStat != nil {
			cpuFrac = calculateCPUFraction(cpuCumulative, ka.prevNodeStat.cpu, nowNano, ka.prevNodeStat.time)
		}
		cpuNano := uint64(cpuFrac * 1000 * 1000 * 1000)

		ka.prevNodeStat = &prevStat{cpu: cpuCumulative, time: nowNano}

		nodeStats.CPU = &statsv1.CPUStats{
			Time:                 nowStamp,
			UsageNanoCores:       &cpuNano,
			UsageCoreNanoSeconds: &cpuCumulative,
		}
	}

	// TODO: cache memory info for 60 seconds or so probably
	if stat, err := linuxproc.ReadMemInfo("/proc/meminfo"); err != nil {
		log.Println("KubeAPI WARN: Failed to read /proc/meminfo:", err.Error())
	} else {
		var availBytes uint64 = stat.MemAvailable * 1024
		var usedBytes uint64 = (stat.MemTotal - stat.MemFree) * 1024
		var workingBytes uint64 = (stat.MemTotal - stat.MemAvailable) * 1024
		var rssBytes uint64 = (stat.AnonPages + stat.AnonHugePages + stat.SwapCached) * 1024
		nodeStats.Memory = &statsv1.MemoryStats{
			Time:            nowStamp,
			AvailableBytes:  &availBytes,
			UsageBytes:      &usedBytes,
			WorkingSetBytes: &workingBytes,
			RSSBytes:        &rssBytes,
		}

		if vmStat, err := linuxproc.ReadVMStat("/proc/vmstat"); err != nil {
			log.Println("KubeAPI WARN: Failed to read /proc/vmstat:", err.Error())
		} else {
			nodeStats.Memory.PageFaults = &vmStat.PageFault
			nodeStats.Memory.MajorPageFaults = &vmStat.PageMajorFault
		}
	}

	if statList, err := linuxproc.ReadNetworkStat("/proc/net/dev"); err != nil {
		log.Println("KubeAPI WARN: Failed to read /proc/net/dev:", err.Error())
	} else {
		netStats := make([]statsv1.InterfaceStats, 0, len(statList))
		for _, stat := range statList {
			if stat.Iface == "lo" || strings.HasPrefix(stat.Iface, "veth") {
				continue
			}

			// not clear if this copying actually prevents anything...
			rxBytes := stat.RxBytes
			txBytes := stat.TxBytes
			rxErrors := stat.RxErrs
			txErrors := stat.TxErrs
			netStats = append(netStats, statsv1.InterfaceStats{
				Name:     stat.Iface,
				RxBytes:  &rxBytes,
				TxBytes:  &txBytes,
				RxErrors: &rxErrors,
				TxErrors: &txErrors,
			})
		}

		if len(netStats) > 0 {
			nodeStats.Network = &statsv1.NetworkStats{
				Time:           nowStamp,
				Interfaces:     netStats,
				InterfaceStats: netStats[0],
			}
		}

	}

	// https://godoc.org/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1#Summary
	return &statsv1.Summary{
		Node: nodeStats,
		Pods: podStats,
	}, nil
}

// https://github.com/containers/podman/blob/3569e24df8c3f774def37d99b7e23158349e92cf/libpod/stats.go#L102
// calculateCPUPercent calculates the cpu usage using the latest measurement in stats.
// previousCPU is the last value of stats.CPU.Usage.Total measured at the time previousSystem.
//  (now - previousSystem) is the time delta in nanoseconds, between the measurement in previousCPU
// and the updated value in stats.
func calculateCPUFraction(nowCPU, previousCPU, nowSystem, previousSystem uint64) float64 {
	var (
		cpuFraction = 0.0
		cpuDelta    = float64(nowCPU - previousCPU)
		systemDelta = float64(nowSystem - previousSystem)
	)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		// gets a ratio of container cpu usage total
		cpuFraction = (cpuDelta / systemDelta)
	}
	return cpuFraction
}
