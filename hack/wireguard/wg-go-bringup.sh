set -eux

wireguard-go -f wg-gke &
sleep 2s

ip addr add 10.6.26.11/32 dev wg-gke # our node
ip addr add 10.10.1.0/25 dev wg-gke # our pods
ip link set dev wg-gke up

wg set wg-gke private-key key
wg set wg-gke peer xxx \
  endpoint wg.xxx:51820 \
  persistent-keepalive 25 \
  allowed-ips 10.6.0.0/20,10.8.0.0/14,10.6.24.0/22

ip route add 10.6.0.0/20 dev wg-gke scope link
ip route add 10.6.24.0/22 dev wg-gke scope link
ip route add 10.8.0.0/14 dev wg-gke scope link
