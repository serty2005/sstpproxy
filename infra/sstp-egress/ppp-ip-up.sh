#!/usr/bin/env bash
set -Eeuo pipefail

IFNAME="${1:-}"
LOCAL_IP="${4:-}"
REMOTE_IP="${5:-}"
IPPARAM="${6:-}"

echo "[ip-up] ifname=${IFNAME} local=${LOCAL_IP} remote=${REMOTE_IP} ipparam=${IPPARAM}"

if [[ "${IFNAME}" != "ppp0" ]]; then
  exit 0
fi

ip rule del pref 100 oif "${IFNAME}" table 100 2>/dev/null || true
ip rule del pref 101 from "${LOCAL_IP}/32" table 100 2>/dev/null || true
ip rule del pref 102 fwmark 0x1 table 100 2>/dev/null || true
ip route flush table 100 2>/dev/null || true

# Трафик, который приложения отправляют через ppp0, идёт через отдельную таблицу.
ip route add default dev "${IFNAME}" src "${LOCAL_IP}" table 100
ip rule add pref 100 oif "${IFNAME}" table 100
ip rule add pref 101 from "${LOCAL_IP}/32" table 100
ip rule add pref 102 fwmark 0x1 table 100

iptables -t mangle -C OUTPUT -m owner --uid-owner 10002 -j MARK --set-mark 0x1 2>/dev/null \
  || iptables -t mangle -A OUTPUT -m owner --uid-owner 10002 -j MARK --set-mark 0x1

iptables -t mangle -C OUTPUT -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1300 2>/dev/null \
  || iptables -t mangle -A OUTPUT -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1300

iptables -t mangle -C FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1300 2>/dev/null \
  || iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --set-mss 1300

ip link set dev "${IFNAME}" mtu 1300 || true

mkdir -p /run
echo "${LOCAL_IP}" > /run/ppp-local-ip
echo "${REMOTE_IP}" > /run/ppp-remote-ip
touch /run/ppp-ready
