package kubeapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	// statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	// utilexec "k8s.io/utils/exec"

	vkapi "github.com/virtual-kubelet/virtual-kubelet/node/api"

	"github.com/danopia/kube-pet-node/controllers/pods"
	"github.com/danopia/kube-pet-node/podman"
)

func MountApi(podManager *pods.PodManager) {
	// api := mux.NewRouter()

	vkapi.AttachPodRoutes(vkapi.PodHandlerConfig{

		// TODO

		RunInContainer: func(ctx context.Context, namespace, podName, containerName string, cmd []string, attach vkapi.AttachIO) error {
			log.Println("RunInContainer(", namespace, podName, containerName, cmd, attach, ")")

			session, err := podManager.StartExecInPod(ctx, pods.PodCoord{namespace, podName}, containerName, &podman.ContainerExecOptions{
				Cmd:          cmd,
				Tty:          attach.TTY(),
				AttachStdin:  attach.Stdin() != nil,
				AttachStdout: attach.Stdout() != nil,
				AttachStderr: attach.Stderr() != nil,
			})
			if err != nil {
				log.Println("exec init err:", err)
				return err
			}

			go func() {
				for termSize := range attach.Resize() {
					log.Println("exec resize:", termSize)
					if err := session.Resize(ctx, &podman.ExecResizeOptions{
						Width:  termSize.Width,
						Height: termSize.Height,
					}); err != nil {
						log.Println("WARN: Resize exec session to", termSize.Width, termSize.Height, "failed:", err)
					}
				}
			}()

			input, output, err := session.Start(ctx)
			if err != nil {
				log.Println("exec start err:", err)
				// TODO: cancel exec?
				return err
			}
			log.Println("exec start done", input, output)

			if input != nil && attach.Stdin() != nil {
				go io.Copy(input, attach.Stdin())
			}

			if attach.TTY() {
				// TODO: why does TTY not have stdout/stderr split??
				io.Copy(attach.Stdout(), output)
			} else {
				podman.DemuxRawStream(output, attach.Stdout(), attach.Stderr(), false)
			}

			if sessResult, err := session.Inspect(ctx); err != nil {
				log.Println("kubeapi WARN: Post-exec inspection failed:", err)
			} else if sessResult.ExitCode > 0 {
				return exitError{sessResult.ExitCode}
			}
			return nil
		},

		GetContainerLogs: func(ctx context.Context, namespace, podName, containerName string, opts vkapi.ContainerLogOpts) (io.ReadCloser, error) {
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

			logs, err := podManager.FetchContainerLogs(ctx, pods.PodCoord{namespace, podName}, containerName, logOpts)
			if err != nil {
				log.Println("logs get err:", err)
				return nil, err
			}

			// kubernetes mixes stdout/stderr so just use one pipe for everything
			outR, outW := io.Pipe()
			go podman.DemuxRawStream(logs, outW, outW, true)
			return outR, nil
		},
	}, mux{}, true)
	// vkapi.AttachPodMetricsRoutes(vkapi.PodMetricsConfig{

	// 	GetStatsSummary: func(context.Context) (*statsv1.Summary, error) {
	// 		log.Println("GetStatsSummary()")
	// 		// https://godoc.org/k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1#Summary
	// 		return &statsv1.Summary{
	// 			Node: statsv1.NodeStats{
	// 				NodeName: "todo",
	// 			},
	// 			Pods: []statsv1.PodStats{

	// 			},
	// 		}, nil
	// 	},

	// }, mux{})

	// srv := &http.Server{
	// 		Handler:      api,
	// 		Addr:         nodeIP.String()+":8000",
	// 		WriteTimeout: 15 * time.Second,
	// 		ReadTimeout:  15 * time.Second,
	// }

	// log.Fatal(http.ListenAndServe(nodeIP.String()+":10250", nil))
}

type mux struct{}

func (m mux) Handle(pattern string, handler http.Handler) {
	http.Handle(pattern, handler)
}

type exitError struct {
	status int
}

func (ee exitError) String() string {
	return fmt.Sprintf("command terminated with exit code %v", ee.status)
}
func (ee exitError) Error() string {
	return fmt.Sprintf("command terminated with exit code %v", ee.status)
}
func (ee exitError) Exited() bool {
	return true
}
func (ee exitError) ExitStatus() int {
	return ee.status
}
