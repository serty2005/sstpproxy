#!/usr/bin/env bash
set -Eeuo pipefail

: "${XRAY_PORT:?}"
: "${MTG_PORT:?}"

ip link show ppp0 >/dev/null 2>&1
ss -lnt | grep -E ":((${XRAY_PORT})|(${MTG_PORT})) " >/dev/null 2>&1
