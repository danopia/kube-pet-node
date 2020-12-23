package kubeapi

import (
	"context"
	"runtime"
	"time"

	statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	// utilexec "k8s.io/utils/exec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ka *KubeApi) GetStatsSummary(ctx context.Context) (*statsv1.Summary, error) {

	numCpus := uint64(runtime.NumCPU())
	nowStamp := metav1.NewTime(time.Now()) // TODO: probably use the SystemNanos number from the stats instead
	var zero uint64

	// generally, the total number of containers we are running will be quite stable
	newStats := make(map[string]prevStat, len(ka.prevStats))

	// the actual API call part
	podReports, err := ka.podManager.GetAllStats(ctx)
	if err != nil {
		return nil, err
	}

	// Round memory stuff so it's in kilobytes
	// I'm doing this to match what real containers seem to look like
	// Can probably undo this if metrics-server-exporter is decommissioned or fixed
	var kbMask uint64 = ^uint64(1023)

	podStats := make([]statsv1.PodStats, 0, len(podReports))
	for podMeta, reports := range podReports {

		var totalNanoCores uint64
		var totalCPUTime uint64
		var totalMem uint64

		var netStats *statsv1.NetworkStats

		conStats := make([]statsv1.ContainerStats, 0, len(reports)-1)
		for conName, report := range reports {

			newStats[report.ContainerID] = prevStat{
				time: report.SystemNano, cpu: report.CPUNano,
			}
			prevStat, hasPrevStat := ka.prevStats[report.ContainerID]

			memUsed := report.MemUsage & kbMask
			memAvail := (report.MemLimit - report.MemUsage) & kbMask

			cpuFrac := 0.0
			if hasPrevStat {
				cpuFrac = calculateCPUFraction(report.CPUNano, prevStat.cpu, report.SystemNano, prevStat.time)
			}
			cpuNano := uint64(cpuFrac*1000*1000*1000) * numCpus // TODO: this is likely still wrong

			totalNanoCores += cpuNano
			totalCPUTime += report.CPUNano
			totalMem += memUsed

			// Maybe use network stats from infra
			if conName == "infra" && (report.NetInput > 0 || report.NetOutput > 0) {
				netStats = &statsv1.NetworkStats{
					Time: nowStamp,
					InterfaceStats: statsv1.InterfaceStats{
						Name:    "default",
						RxBytes: &report.NetInput,
						TxBytes: &report.NetOutput,
					},
				}
			}

			// Don't return infra containers themselves
			if conName != "_infra" {
				conStats = append(conStats, statsv1.ContainerStats{
					Name: conName,

					StartTime: nowStamp,
					CPU: &statsv1.CPUStats{
						Time:                 nowStamp,
						UsageNanoCores:       &cpuNano,
						UsageCoreNanoSeconds: &report.CPUNano,
					},
					Memory: &statsv1.MemoryStats{
						Time:           nowStamp,
						AvailableBytes: &memAvail,
						UsageBytes:     &memUsed,
						// WorkingSetBytes: &memUsed,
						// RSSBytes:        &memUsed,
					},
					Rootfs: &statsv1.FsStats{
						Time:           nowStamp,
						AvailableBytes: &zero,
						CapacityBytes:  &zero,
						UsedBytes:      &zero,
					},
					Logs: &statsv1.FsStats{
						Time:           nowStamp,
						AvailableBytes: &zero,
						CapacityBytes:  &zero,
						UsedBytes:      &zero,
					},
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
				Time:           nowStamp,
				AvailableBytes: &zero,
				UsageBytes:     &totalMem,
				// WorkingSetBytes: &totalMem,
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

	var filler uint64 = 0 // TODO: alll the node stats
	// https://godoc.org/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1#Summary
	return &statsv1.Summary{
		Node: statsv1.NodeStats{
			NodeName:  ka.nodeName,
			StartTime: nowStamp,
			CPU: &statsv1.CPUStats{
				Time:                 nowStamp,
				UsageNanoCores:       &filler,
				UsageCoreNanoSeconds: &filler,
			},
			Memory: &statsv1.MemoryStats{
				Time:           nowStamp,
				AvailableBytes: &filler,
				UsageBytes:     &filler,
				// WorkingSetBytes: &filler,
				// RSSBytes:        &filler,
			},
		},
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
