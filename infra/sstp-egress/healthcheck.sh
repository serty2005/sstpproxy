#!/usr/bin/env bash
set -Eeuo pipefail

PPP_LOCAL_IP="$(tr -d '\r\n' < /run/ppp-local-ip)"

ip -o link show dev ppp0 | grep -Eq '<[^>]*UP[^>]*>'
ip route show table 100 | grep -Eq '^default dev ppp0( |$)'
ip rule show | grep -Eq '(^|[[:space:]])100:.* oif ppp0( .*)? lookup 100|pref 100 .* oif ppp0( .*)? lookup 100'
ip rule show | grep -Eq '(^|[[:space:]])101:|pref 101 ' >/dev/null
ip rule show | grep -Fq "from ${PPP_LOCAL_IP} lookup 100" || ip rule show | grep -Fq "from ${PPP_LOCAL_IP}/32 lookup 100"
ip route get 1.1.1.1 from "${PPP_LOCAL_IP}" | grep -q 'dev ppp0'

egress_ok=0
for url in \
  https://www.cloudflare.com/cdn-cgi/trace \
  https://connectivitycheck.gstatic.com/generate_204
do
  if curl --fail --silent --show-error \
    --connect-timeout 3 \
    --max-time 5 \
    --interface "${PPP_LOCAL_IP}" \
    "${url}" \
    -o /dev/null
  then
    egress_ok=1
    break
  fi
done

[[ "${egress_ok}" -eq 1 ]]
