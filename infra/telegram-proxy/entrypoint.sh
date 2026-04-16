#!/bin/sh
set -eu

log() {
  echo "[$(date '+%F %T')] $*"
}

config_path="/tmp/tinyproxy.conf"
cp /etc/tinyproxy/tinyproxy.conf "$config_path"

ppp_ip=""
for _ in $(seq 1 60); do
  ppp_ip="$(ifconfig ppp0 2>/dev/null | awk '/inet / {value=$2; sub(/^addr:/, "", value); print value; exit}')"
  if [ -n "$ppp_ip" ]; then
    break
  fi
  sleep 1
done

if [ -z "$ppp_ip" ]; then
  log "Ошибка: не удалось определить IPv4-адрес ppp0"
  exit 1
fi

printf '\nBind %s\n' "$ppp_ip" >> "$config_path"
log "Запускаю tinyproxy с исходящим Bind=${ppp_ip}"

exec tinyproxy -d -c "$config_path"
