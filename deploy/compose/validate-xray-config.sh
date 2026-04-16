#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${SCRIPT_DIR}/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-${SCRIPT_DIR}/.env}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Ошибка: env-файл не найден: ${ENV_FILE}" >&2
  exit 1
fi

docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" ps control-plane >/dev/null
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" exec -T control-plane \
  xray run -test -config /srv/xray/generated/config.json

echo "Валидация generated Xray config завершилась успешно."
