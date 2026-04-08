#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
ENV_FILE="${ENV_FILE:-${SCRIPT_DIR}/.env}"
CONFIG_PATH="${CONFIG_PATH:-${REPO_ROOT}/infra/xray/generated/config.json}"
PYTHON_BIN="${PYTHON_BIN:-}"

if [[ -z "${PYTHON_BIN}" ]]; then
  if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN="python3"
  else
    PYTHON_BIN="python"
  fi
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Ошибка: env-файл не найден: ${ENV_FILE}" >&2
  exit 1
fi

if [[ ! -s "${CONFIG_PATH}" ]]; then
  echo "Ошибка: активный Xray-конфиг не найден или пуст: ${CONFIG_PATH}" >&2
  exit 1
fi

XRAY_VERSION="$(
  "${PYTHON_BIN}" - "${ENV_FILE}" <<'PY'
from __future__ import annotations

import sys
from pathlib import Path

env_path = Path(sys.argv[1])
for raw_line in env_path.read_text(encoding="utf-8").splitlines():
    line = raw_line.strip()
    if not line or line.startswith("#") or "=" not in line:
        continue
    key, value = line.split("=", 1)
    if key.strip() == "XRAY_VERSION":
        print(value.strip())
        raise SystemExit(0)

raise SystemExit("XRAY_VERSION не найден в env-файле")
PY
)"

docker run --rm \
  -v "${CONFIG_PATH}:/etc/xray/config.json:ro" \
  "ghcr.io/xtls/xray-core:${XRAY_VERSION}" \
  run -test -config /etc/xray/config.json

echo "Валидация generated Xray config завершилась успешно."
