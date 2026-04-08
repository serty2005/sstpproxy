#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  echo "[$(date '+%F %T')] $*"
}

cleanup() {
  log "Останавливаю контейнер sstp-egress"
  if [[ -n "${SSTPC_PID:-}" ]] && kill -0 "${SSTPC_PID}" 2>/dev/null; then
    kill "${SSTPC_PID}" 2>/dev/null || true
  fi
  wait || true
}

trap cleanup EXIT INT TERM

: "${REMOTEHOST:?Требуется REMOTEHOST}"
: "${USER:?Требуется USER}"
: "${PASSWORD:?Требуется PASSWORD}"

if [[ ! -c /dev/ppp ]]; then
  log "Ошибка: устройство /dev/ppp недоступно внутри контейнера"
  exit 1
fi

mkdir -p /run
rm -f /run/ppp-ready /run/ppp-local-ip /run/ppp-remote-ip

for f in \
  /proc/sys/net/ipv4/conf/all/rp_filter \
  /proc/sys/net/ipv4/conf/default/rp_filter \
  /proc/sys/net/ipv4/conf/eth0/rp_filter
do
  if [[ -w "$f" ]]; then
    echo 0 > "$f" || true
  fi
done

log "Текущие маршруты перед запуском SSTP"
ip route || true
log "Текущие policy rules перед запуском SSTP"
ip rule || true

SSTPC_ARGS=(
  "--user" "$USER"
  "--password" "$PASSWORD"
  "$REMOTEHOST"
  "--log-stdout"
  "--log-level" "4"
  "--tls-ext"
  "noauth"
  "ipparam" "sstp-vpn"
  "nodefaultroute"
  "noipdefault"
  "nobsdcomp"
  "nodeflate"
  "novj"
  "mtu" "1350"
  "mru" "1350"
)

log "Запускаю sstp-client"
sstpc "${SSTPC_ARGS[@]}" &
SSTPC_PID=$!

log "Ожидаю готовность ppp0 через ip-up hook"
for _ in $(seq 1 60); do
  if ! kill -0 "$SSTPC_PID" 2>/dev/null; then
    log "Ошибка: sstpc/pppd завершился до готовности ppp0"
    wait "$SSTPC_PID" || true
    exit 1
  fi

  if [[ -f /run/ppp-ready ]]; then
    break
  fi

  sleep 1
done

if [[ ! -f /run/ppp-ready ]]; then
  log "Ошибка: ppp0 не стал готов за 60 секунд"
  exit 1
fi

PPP_IP="$(cat /run/ppp-local-ip)"
PPP_PEER_IP="$(cat /run/ppp-remote-ip)"

log "VPN готов. ppp0 local=${PPP_IP}, peer=${PPP_PEER_IP}"
log "Маршруты после запуска SSTP"
ip route || true
log "Policy rules после запуска SSTP"
ip rule || true

wait "$SSTPC_PID"
