package kubeapi

import (
	// "net"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	// statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"

	vkapi "github.com/virtual-kubelet/virtual-kubelet/node/api"
)

func MountApi() {
	// api := mux.NewRouter()

	vkapi.AttachPodRoutes(vkapi.PodHandlerConfig{

		// TODO

		RunInContainer: func(ctx context.Context, namespace, podName, containerName string, cmd []string, attach vkapi.AttachIO) error {
			log.Println("RunInContainer(", namespace, podName, containerName, cmd, attach, ")")
			attach.Stdout().Write([]byte("hi world"))
			attach.Stdout().Close()
			return nil
		},

		GetContainerLogs: func(ctx context.Context, namespace, podName, containerName string, opts vkapi.ContainerLogOpts) (io.ReadCloser, error) {
			log.Println("GetContainerLogs(", namespace, podName, containerName, opts, ")")
			log.Printf("%+v", opts)
			// https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node/api#ContainerLogOpts
			return ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil
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
