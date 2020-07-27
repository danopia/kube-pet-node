#!/bin/sh
getent group kube-pet >/dev/null || groupadd -r kube-pet
getent passwd kube-pet >/dev/null || \
  useradd -r -g kube-pet -d /opt/kube-pet-node -s /sbin/nologin \
  -c "Account for kube-pet-node to run as" kube-pet

if [ -d /etc/wireguard ]; then
  chown -R :kube-pet /etc/wireguard
  chmod -R g+rw /etc/wireguard
fi

if ! [ -d /opt/kube-pet-node/.cache ]; then
  mkdir /opt/kube-pet-node/.cache
  chown -R kube-pet:kube-pet /opt/kube-pet-node/.cache
  chmod -R 0700 /opt/kube-pet-node/.cache
fi
