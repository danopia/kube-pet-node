package kubeapi

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// certv1 "k8s.io/api/certificates/v1beta1"
	"k8s.io/client-go/kubernetes"

	vkapi "github.com/virtual-kubelet/virtual-kubelet/node/api"

	"github.com/danopia/kube-pet-node/controllers/pods"
)

type KubeApi struct {
	keyStorage *KeyMaterialStorage
	kubernetes *kubernetes.Clientset
	podManager *pods.PodManager
	nodeName   string
	nodeIP     net.IP

	httpSrv  *http.Server
	httpLnr  net.Listener
	httpsSrv *http.Server
	httpsLnr net.Listener
}

func NewKubeApi(kubernetes *kubernetes.Clientset, podManager *pods.PodManager, nodeName string, nodeIP net.IP) (*KubeApi, error) {

	if nodeIP == nil {
		log.Println("WARN: kubeapi listening on all interfaces")
		nodeIP = net.ParseIP("0.0.0.0")
	}

	keyStorage, err := NewKeyMaterialStorage("kubelet-server")
	if err != nil {
		return nil, err
	}

	httpSrv := &http.Server{Addr: nodeIP.String() + ":10255"}
	httpLnr, err := net.Listen("tcp", httpSrv.Addr)
	if err != nil {
		return nil, err
	}

	httpsSrv := &http.Server{Addr: nodeIP.String() + ":10250"}
	httpsLnr, err := net.Listen("tcp", httpsSrv.Addr)
	if err != nil {
		return nil, err
	}

	return &KubeApi{
		keyStorage: keyStorage,
		kubernetes: kubernetes,
		podManager: podManager,
		nodeName:   nodeName,
		nodeIP:     nodeIP,

		httpSrv:  httpSrv,
		httpLnr:  httpLnr,
		httpsSrv: httpsSrv,
		httpsLnr: httpsLnr,
	}, nil
}

func (ka *KubeApi) Run(ctx context.Context) {
	defer ka.httpsLnr.Close()

	secureMux := http.NewServeMux()
	vkapi.AttachPodRoutes(vkapi.PodHandlerConfig{
		RunInContainer:   ka.RunInContainer,
		GetContainerLogs: ka.GetContainerLogs,
		GetStatsSummary:  ka.GetStatsSummary,
	}, secureMux, true)
	ka.httpsSrv.Handler = secureMux

	insecureMux := http.NewServeMux()
	vkapi.AttachPodMetricsRoutes(vkapi.PodMetricsConfig{
		GetStatsSummary: ka.GetStatsSummary,
	}, insecureMux)
	ka.httpSrv.Handler = insecureMux

	// TODO: close servers when ctx cancels

	// get HTTP going immediately / in the background
	go func() {
		defer ka.httpLnr.Close()
		log.Fatalln(ka.httpSrv.Serve(ka.httpLnr))
	}()

	// try HTTPS once, half-expecting the key files to be missing
	err := ka.httpsSrv.ServeTLS(
		ka.httpsLnr,
		ka.keyStorage.GetFilePath(".crt"),
		ka.keyStorage.GetFilePath(".key"))

	// from here down is only reached if the first TLS listen attempt failed
	// fail hard if it wasn't expected
	if !os.IsNotExist(err) {
		log.Fatalln(err)
	}

	if err := ka.PerformCertificateFlow(ctx); err != nil {
		log.Fatalln(err)
	}

	// try HTTPS once more, fail hard if not
	log.Fatalln(ka.httpsSrv.ServeTLS(
		ka.httpsLnr,
		ka.keyStorage.GetFilePath(".crt"),
		ka.keyStorage.GetFilePath(".key")))
}
