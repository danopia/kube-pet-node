#!/bin/sh
set -e
# this file is based on mongod's postinst

case "$1" in
    configure)
        # create a kube-pet group and user
        if ! getent passwd kube-pet >/dev/null; then
                adduser --system --disabled-password --disabled-login \
                        --home /opt/kube-pet-node --no-create-home \
                        --quiet --group kube-pet
        fi

        # allow directly managing wireguard configs
        if ! dpkg-statoverride --list "/etc/wireguard" >/dev/null 2>&1; then
                dpkg-statoverride --update --add root kube-pet 0775 "/etc/wireguard"
        fi
        if [ -d /etc/wireguard ]; then
                chown -R :kube-pet /etc/wireguard
                chmod -R g+rwx /etc/wireguard
        fi

        # give service a private read/write space
        if ! [ -d /opt/kube-pet-node/.cache ]; then
                mkdir /opt/kube-pet-node/.cache
                chown -R kube-pet:kube-pet /opt/kube-pet-node/.cache
                chmod -R 0700 /opt/kube-pet-node/.cache
        fi

        # allow managing the network without sudo
        setcap cap_net_admin+ep "$(readlink /usr/bin/kube-pet-node)"
    ;;

    abort-upgrade|abort-remove|abort-deconfigure)
    ;;

    *)
        echo "postinst called with unknown argument \`$1'" >&2
        exit 1
    ;;
esac

# (Not) Automatically added by dh_installsystemd/11.1.6ubuntu2
if [ "$1" = "configure" ] || [ "$1" = "abort-upgrade" ] || [ "$1" = "abort-deconfigure" ] || [ "$1" = "abort-remove" ] ; then
        # This will only remove masks created by d-s-h on package removal.
        deb-systemd-helper unmask 'kube-pet-node.service' >/dev/null || true
        deb-systemd-helper unmask 'kube-podman.service' >/dev/null || true
        deb-systemd-helper unmask 'kube-podman.socket' >/dev/null || true

        if deb-systemd-helper --quiet was-enabled 'kube-podman.service'; then
                deb-systemd-helper enable 'kube-podman.service' >/dev/null || true
        else
                deb-systemd-helper update-state 'kube-podman.service' >/dev/null || true
        fi
        if deb-systemd-helper --quiet was-enabled 'kube-podman.socket'; then
                deb-systemd-helper enable 'kube-podman.socket' >/dev/null || true
                systemctl daemon-reload && systemctl restart 'kube-podman.socket' || true
        else
                deb-systemd-helper update-state 'kube-podman.socket' >/dev/null || true
        fi

        # was-enabled defaults to true, so new installations run enable.
        if deb-systemd-helper --quiet was-enabled 'kube-pet-node.service'; then
                # Enables the unit on first installation, creates new
                # symlinks on upgrades if the unit file has changed.
                deb-systemd-helper enable 'kube-pet-node.service' >/dev/null || true
                systemctl daemon-reload && systemctl restart 'kube-pet-node.service' || true
        else
                # Update the statefile to add new symlinks (if any), which need to be
                # cleaned up on purge. Also remove old symlinks.
                deb-systemd-helper update-state 'kube-pet-node.service' >/dev/null || true
        fi
fi
# End (not) automatically added section

exit 0
