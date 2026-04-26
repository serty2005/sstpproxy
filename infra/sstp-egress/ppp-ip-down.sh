#!/usr/bin/env bash
set -Eeuo pipefail

IFNAME="${1:-}"
LOCAL_IP="${4:-}"
MTG_UID="${MTG_UID:-10002}"
MTG_PORT="${MTG_PORT:-4430}"

echo "[ip-down] ifname=${IFNAME} local=${LOCAL_IP}"

if [[ "${IFNAME}" != "ppp0" ]]; then
  exit 0
fi

ip rule del pref 100 oif "${IFNAME}" table 100 2>/dev/null || true
ip rule del pref 101 from "${LOCAL_IP}/32" table 100 2>/dev/null || true
ip rule del pref 102 fwmark 0x1 table 100 2>/dev/null || true
ip route flush table 100 2>/dev/null || true

iptables -t mangle -D OUTPUT -m owner --uid-owner "${MTG_UID}" -j MARK --set-mark 0x1 2>/dev/null || true
iptables -t mangle -D OUTPUT -p tcp -m tcp ! --sport "${MTG_PORT}" -m owner --uid-owner "${MTG_UID}" -j MARK --set-mark 0x1 2>/dev/null || true
iptables -t nat -D POSTROUTING -m mark --mark 0x1 -o "${IFNAME}" -j SNAT --to-source "${LOCAL_IP}" 2>/dev/null || true

rm -f /run/ppp-ready /run/ppp-local-ip /run/ppp-remote-ip
