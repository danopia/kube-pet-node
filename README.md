# kube-pet-node

Proof of Concept

```
dan@penguin ~> kubectl get nodes
NAME                               STATUS   ROLES    AGE     VERSION
laxbox                             Ready    <none>   11h     metal/v0.1.0
atlbox                             Ready    <none>   45m     metal/v0.1.0
gke-dust-persist3-190b8935-jvph    Ready    <none>   89d     v1.16.8-gke.15
gke-dust-volatile1-b331c9e9-fk37   Ready    <none>   2d16h   v1.16.8-gke.15
```

The concept is taking a managed cloud-based control plane and adding external bare-metal servers to it. I'm developing against a free GKE control plane and

## deps

Current plans are to use all of these programs on the pet node:

* wireguard - tunnelling between the 'real' cluster and the pet node
* nftables (via [library](https://github.com/google/nftables)) - configure traffic [for ClusterIP services](https://wiki.nftables.org/wiki-nftables/index.php/Load_balancing#Round_Robin), maybe also network policy
* podman (via new REST API in 2.x) - run 'pods' on the host (similar to docker, but with its own pod concept)

and also have support for these extras:

* zfs - report health and status of storage pools as a CRD, also probably allocation
* smartctl - report health of storage devices as a CRD
* systemd - used for scheduling pods and tunnels for server boots
  * goal of the system being able to come up and serve Internet apps without kubernetes/kube-pet operational
* journald - used internally by podman for storing/watching state changes
