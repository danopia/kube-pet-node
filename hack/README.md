A new node needs these manual steps:

1. Install wireguard, nftables, podman 2+
1. Configure a wireguard tunnel
  1. `wg genkey` and `wg pubkey`
  1. Add a peer to your server with a unique node IP and pod CIDR
    1. `systemctl restart wg-quick@wg-gke`
  1. Create `/etc/wireguard/wg-gke.conf`
    1. Use `hack/wireguard-pet-node.conf` as a guide
  1. Bring up wg on your pet
    1. `systemctl enable --now wg-quick@wg-gke`
  1. Check health: `wg show wg-gke`
1. Configure a CNI for the pod CIDR
  1. Copy `hack/cni/51-kube-net.conflist` to `/etc/cni/net.d/`
  1. Update the 'subnet'
1. Pick a hostname
1. Run `cd node-bootstrap; ./upsert-node.sh <hostname>` on your laptop
1. Start the pet
  1. Manually:
    1. Copy generated node-kubeconfig.yaml to the pet, where `kube-pet-node` is
    1. On your pet, run `kube-pet-node`
  1. Using a package:
    1. Install the latest deb or rpm manually
    1. Copy generated node-kubeconfig.yaml to `/opt/kube-pet-node`
    1. Set kubeconfig ownership to `kube-pet:kube-pet` and make sure it's chmod 0600 or similar
    1. Wait a minute for the service to restart and pick up the kubeconfig
1. Confirm presence in `kubectl get nodes` on your laptop
1. The pet logs a `kubectl certificate approve` command on first run - so run that from your laptop
1. You should now be clear to schedule pods onto your pet :)

"Your laptop" refers to whatever machine you have with admin access to the Kubernetes cluster.
