#!/bin/sh
if [ $1 -eq 0 ] && [ -x /usr/bin/systemctl ] ; then
  # Package removal, not upgrade
  /usr/bin/systemctl --no-reload disable --now kube-pet-node.service >/dev/null 2>&1 || :
  /usr/bin/systemctl --no-reload disable --now kube-podman.socket >/dev/null 2>&1 || :
  /usr/bin/systemctl --no-reload disable --now kube-podman.service >/dev/null 2>&1 || :
fi
