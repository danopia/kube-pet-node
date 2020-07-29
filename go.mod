module github.com/danopia/kube-pet-node

go 1.13

replace (
	k8s.io/api => k8s.io/api v0.16.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.8
	k8s.io/apiserver => k8s.io/apiserver v0.16.8
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.8
	k8s.io/client-go => k8s.io/client-go v0.16.8
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.16.8
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.8
	k8s.io/code-generator => k8s.io/code-generator v0.16.8
	k8s.io/component-base => k8s.io/component-base v0.16.8
	k8s.io/cri-api => k8s.io/cri-api v0.16.8
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.16.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.16.8
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.16.8
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.16.8
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.16.8
	k8s.io/kubectl => k8s.io/kubectl v0.16.8
	k8s.io/kubelet => k8s.io/kubelet v0.16.8
	k8s.io/kubernetes => k8s.io/kubernetes v1.16.8
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.16.8
	k8s.io/metrics => k8s.io/metrics v0.16.8
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.16.8
)

require (
	github.com/bep/debounce v1.2.0
	github.com/containers/libpod v1.9.3
	github.com/coreos/go-semver v0.3.0
	github.com/go-ini/ini v1.57.0
	github.com/google/nftables v0.0.0-20200316075819-7127d9d22474
	github.com/gorilla/mux v1.7.4
	github.com/pbnjay/memory v0.0.0-20190104145345-974d429e7ae4
	github.com/virtual-kubelet/virtual-kubelet v0.0.0-20200504180557-8fc8b69d8f53
	golang.org/x/sys v0.0.0-20200327173247-9dae0f8f5775
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/kubernetes v1.15.2
)
