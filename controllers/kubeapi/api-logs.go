package kubeapi

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"strconv"

	// utilexec "k8s.io/utils/exec"

	vkapi "github.com/virtual-kubelet/virtual-kubelet/node/api"

	"github.com/danopia/kube-pet-node/controllers/pods"
	"github.com/danopia/kube-pet-node/podman"
)

func (ka *KubeApi) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts vkapi.ContainerLogOpts) (io.ReadCloser, error) {
	log.Println("GetContainerLogs(", namespace, podName, containerName, opts, ")")
	log.Printf("%+v", opts)

	// https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node/api#ContainerLogOpts
	if opts.Previous {
		return ioutil.NopCloser(bytes.NewReader([]byte("TODO: kube-pet-node doesn't support --previous=true"))), nil
	}
	logOpts := &podman.ContainerLogsOptions{
		Timestamps: opts.Timestamps,
		Follow:     opts.Follow,
	}
	// TODO: LimitBytes   int
	// TODO: SinceSeconds int
	// TODO: SinceTime    time.Time
	if opts.Tail > 0 {
		logOpts.Tail = strconv.Itoa(opts.Tail)
	}

	logs, err := ka.podManager.FetchContainerLogs(ctx, pods.PodCoord{namespace, podName}, containerName, logOpts)
	if err != nil {
		log.Println("logs get err:", err)
		return nil, err
	}

	// kubernetes mixes stdout/stderr so just use one pipe for everything
	outR, outW := io.Pipe()
	go podman.DemuxRawStream(logs, outW, outW, true)
	return outR, nil
}
