#!/bin/sh
if [ $1 -eq 1 ] && [ -x /usr/bin/systemctl ] ; then
  # Initial installation
  /usr/bin/systemctl enable kube-podman.service >/dev/null 2>&1 || :
  /usr/bin/systemctl enable --now kube-podman.socket >/dev/null 2>&1 || :
  /usr/bin/systemctl enable --now kube-pet-node.service >/dev/null 2>&1 || :
fi
