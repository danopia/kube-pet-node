# kube-pet-node

Proof of Concept

```
dan@penguin ~> kubectl get nodes
NAME                               STATUS   ROLES    AGE     VERSION
pet-laxbox                         Ready    pet      11h     metal/v0.1.0
pet-atlbox                         Ready    pet      45m     metal/v0.1.0
gke-dust-persist3-190b8935-jvph    Ready    app      89d     v1.16.8-gke.15
gke-dust-volatile1-b331c9e9-fk37   Ready    app      2d16h   v1.16.8-gke.15
```

The concept is taking a managed cloud-based control plane and adding external bare-metal servers to it to be maintained as pets. I'm developing against a free GKE control plane using a Public control plane and Alias IP pod networking.

## goalposts
I'm just tryina run some high-resource stuff out of the cloud so it's cheaper.

### v1 goals
* [ ] learn a shitton about what kubelet actually does and how a Kubernetes node works
* [ ] run reasonably sane Kubernetes pods
* [x] request our own api certificates automatically
* provide interactive container apis:
  * [x] exec
  * [x] logs
  * [ ] attach
  * [x] metrics
* [ ] expose static pod representing the host system (host exec & dmesg logs)
  * (static pods have annotations incl. `kubernetes.io/config.source`)
* [ ] emit Event resources just like kubelet
  * [x] image pulling
  * [ ] container lifecycle
* [ ] image pull backoff
* [ ] restart failed/finished containers
* [x] support imagepullsecrets
* [x] report our Internet address in node status (for dynamic dns purposes)
* [x] require as few permissions as possible - non-root, plus CAP_NET_ADMIN and access to a root podman
* [ ] reasonable volume support
  * [x] HostPath volumes
  * [ ] ConfigMap volumes
  * [ ] Secret volumes
  * [ ] EmptyDir volumes
  * [ ] NFS volumes
* [ ] support downward IP in envars
* [x] include build version in our Node resources
* [x] support self-upgrading the node itself, on demand (e.g. update target version in a configmap)

### stretch goals
* report pod and node metrics
* update mounted configmaps/secrets in pods
* support init containers on pods (and eventually ephemeral containers)
* support container readiness and liveness probes
* kubectl port-forward
* create CRDs to observe and maybe manipulate hardware devices (disk drives, TV tuners, etc)
  * loops probably distributed as a daemonset/deployment, even if it made in-project
* support drone.io job pods (changing image of running containers)
* support NetworkPolicy on pod networking
* support registering pods into systemctl for bringing up at boot, even if kube-pet-node is broken
* support running entirely rootless (incl. rootless podman), document the limitations (no pod IPs, probably no clusterips, etc)

### non goals
* \[not\] matching container lifecycle (ok to restart a container instead of replacing each time)
* \[not\] feature compatible with kube-proxy (e.g. completely ignoring NodePort/LoadBalancer)
* \[not\] supporting alternative CNI or CSI or CRI configurations (sticking with ptp/wg, hostpath, & podman for now)

## deps

Current plans are to use all of these programs on the ideal pet node, though none are mission critical to have.

* wireguard - tunnelling between the 'real' cluster and the pet node - you need some sort of networking for pods to talk properly, can be another VPN though
* nftables (via [library](https://github.com/google/nftables)) - configure traffic [for ClusterIP services](https://wiki.nftables.org/wiki-nftables/index.php/Load_balancing#Round_Robin), maybe also network policy - cluster IPs won't exist without nftables
* podman (via REST API, new in 2.x) - run 'pods' on the host (similar to docker, but with its own pod concept) - required to run pods

opt-in extras for later:

* zfs - report health and status of storage pools as a CRD, also probably volume provisioning
* smartctl - report health of storage devices as a CRD
* systemd - used for scheduling pods and tunnels for server boots
  * goal of the system being able to come up and serve Internet apps without kubernetes/kube-pet operational
* journald - used internally by podman for storing/watching state changes
