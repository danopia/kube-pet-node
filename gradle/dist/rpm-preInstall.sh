#!/bin/sh
getent group kube-pet >/dev/null || groupadd -r kube-pet
getent passwd kube-pet >/dev/null || \
  useradd -r -g kube-pet -d /opt/kube-pet-node -s /sbin/nologin \
  -c "Account for kube-pet-node to run as" kube-pet
