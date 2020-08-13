package kubeapi

import (
	"context"
	"strings"
	"time"

	statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	// utilexec "k8s.io/utils/exec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ka *KubeApi) GetStatsSummary(ctx context.Context) (*statsv1.Summary, error) {

	nowStamp := metav1.NewTime(time.Now())
	var zero uint64

	// the actual API call part
	podReports, err := ka.podManager.GetAllStats(ctx)
	if err != nil {
		return nil, err
	}

	podStats := make([]statsv1.PodStats, 0, len(podReports))
	for podMeta, reports := range podReports {

		var totalCpu uint64
		var totalMem uint64

		conStats := make([]statsv1.ContainerStats, 0, len(reports)-1)
		for _, report := range reports {

			conName := "_"
			nameParts := strings.Split(report.Name, "_")
			if len(nameParts) == 3 {
				conName = nameParts[2]
			}

			memUsed, memAvail, _ := report.ReadMemUsage()
			cpuPerc, _ := report.ReadCpuPercent()
			cpuNano := uint64(cpuPerc * 100000)

			totalCpu += cpuNano
			totalMem += memUsed

			// Don't return infra containers themselves
			if conName != "_" {
				conStats = append(conStats, statsv1.ContainerStats{
					Name: conName,

					StartTime: nowStamp,
					CPU: &statsv1.CPUStats{
						Time:                 nowStamp,
						UsageNanoCores:       &cpuNano,
						UsageCoreNanoSeconds: &cpuNano,
					},
					Memory: &statsv1.MemoryStats{
						Time:            nowStamp,
						AvailableBytes:  &memAvail,
						UsageBytes:      &memUsed,
						WorkingSetBytes: &memUsed,
						RSSBytes:        &memUsed,
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
				UsageNanoCores:       &totalCpu,
				UsageCoreNanoSeconds: &zero,
			},
			Memory: &statsv1.MemoryStats{
				Time:            nowStamp,
				AvailableBytes:  &zero,
				UsageBytes:      &totalMem,
				WorkingSetBytes: &totalMem,
				RSSBytes:        &totalMem,
			},
			Network: &statsv1.NetworkStats{
				Time: nowStamp,
				// 	InterfaceStats: statsv1.InterfaceStats{
				// 		Name:    "hi",
				// 		RxBytes: &zero,
				// 		TxBytes: &zero,
				// 	},
			},
			EphemeralStorage: &statsv1.FsStats{
				Time:           nowStamp,
				AvailableBytes: &zero,
				CapacityBytes:  &zero,
				UsedBytes:      &zero,
			},
			// ProcessStats: &statsv1.ProcessStats{
			// 	ProcessCOunt: &zero,
			// },
		})
	}

	var filler uint64 = 69000000
	// https://godoc.org/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1#Summary
	return &statsv1.Summary{
		Node: statsv1.NodeStats{
			NodeName:  "pet-penguin",
			StartTime: nowStamp,
			CPU: &statsv1.CPUStats{
				Time:                 nowStamp,
				UsageNanoCores:       &filler,
				UsageCoreNanoSeconds: &filler,
			},
			Memory: &statsv1.MemoryStats{
				Time:            nowStamp,
				AvailableBytes:  &filler,
				UsageBytes:      &filler,
				WorkingSetBytes: &filler,
				RSSBytes:        &filler,
			},
		},
		Pods: podStats,
	}, nil
}
