#!/bin/sh
if [ $1 -eq 1 ] && [ -x /usr/bin/systemctl ] ; then
  # Initial installation

  # allow directly managing wireguard configs
  if [ -d /etc/wireguard ]; then
    chown -R :kube-pet /etc/wireguard
    chmod -R g+rw /etc/wireguard
  fi

  # give service a private read/write space
  if ! [ -d /opt/kube-pet-node/.cache ]; then
    mkdir /opt/kube-pet-node/.cache
    chown -R kube-pet:kube-pet /opt/kube-pet-node/.cache
    chmod -R 0700 /opt/kube-pet-node/.cache
  fi

  /usr/bin/systemctl enable kube-podman.service >/dev/null 2>&1 || :
  /usr/bin/systemctl enable --now kube-podman.socket >/dev/null 2>&1 || :
  /usr/bin/systemctl enable --now kube-pet-node.service >/dev/null 2>&1 || :
fi

# allow managing the network without sudo
setcap cap_net_admin+ep "$(readlink /usr/bin/kube-pet-node)"
