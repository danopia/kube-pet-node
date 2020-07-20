package kubeapi

import (
	// "net"
	"net/http"
	// "log"

	vkapi "github.com/virtual-kubelet/virtual-kubelet/node/api"
)

func MountApi() {
	// api := mux.NewRouter()
	vkapi.AttachPodRoutes(vkapi.PodHandlerConfig{
		// TODO
	}, mux{}, true)
	// srv := &http.Server{
	// 		Handler:      api,
	// 		Addr:         nodeIP.String()+":8000",
	// 		WriteTimeout: 15 * time.Second,
	// 		ReadTimeout:  15 * time.Second,
	// }

	// log.Fatal(http.ListenAndServe(nodeIP.String()+":10250", nil))
}

type mux struct {}
func (m mux) Handle(pattern string, handler http.Handler) {
	http.Handle(pattern, handler)
}
