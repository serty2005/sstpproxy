#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${SCRIPT_DIR}/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-${SCRIPT_DIR}/.env}"
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

"${PYTHON_BIN}" - "${ENV_FILE}" <<'PY'
from __future__ import annotations

import sys
from pathlib import Path

env_path = Path(sys.argv[1])


def parse_env(path: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            raise SystemExit(f"Ошибка: строка в env без '=': {raw_line}")
        key, value = line.split("=", 1)
        result[key.strip()] = value.strip()
    return result


def fail(message: str) -> None:
    print(f"Ошибка: {message}", file=sys.stderr)
    raise SystemExit(1)


def warn(message: str) -> None:
    print(f"Предупреждение: {message}")


env = parse_env(env_path)

required_vars = [
    "SSTP_EGRESS_IMAGE",
    "TELEGRAM_PROXY_IMAGE",
    "CONTROL_PLANE_IMAGE",
    "SSTP_REMOTEHOST",
    "SSTP_USERNAME",
    "SSTP_PASSWORD",
    "PUBLIC_HOST",
    "XRAY_PORT",
    "MTG_PORT",
    "CONTROL_PLANE_PORT",
    "XRAY_REALITY_DEST",
    "XRAY_REALITY_SERVER_NAMES",
    "POSTGRES_DB",
    "POSTGRES_USER",
    "POSTGRES_PASSWORD",
    "MTPROTO_SECRET_VALUE",
    "XRAY_VERSION",
    "XRAY_IMAGE_TAG",
    "MTG_IMAGE_TAG",
]

for key in required_vars:
    if not env.get(key):
        fail(f"не задана обязательная переменная {key}")

for key in ("XRAY_PORT", "MTG_PORT", "CONTROL_PLANE_PORT"):
    value = env[key]
    if not value.isdigit():
        fail(f"{key} должен быть целым числом")
    port = int(value)
    if not 1 <= port <= 65535:
        fail(f"{key} должен быть в диапазоне 1..65535")

if not env["XRAY_VERSION"].startswith("v"):
    fail("XRAY_VERSION должен быть в формате release tag, например v26.3.27")

if env["XRAY_IMAGE_TAG"].startswith("v"):
    fail("XRAY_IMAGE_TAG должен быть без префикса 'v', например 26.3.27")

if not [part.strip() for part in env["XRAY_REALITY_SERVER_NAMES"].split(",") if part.strip()]:
    fail("XRAY_REALITY_SERVER_NAMES должен содержать хотя бы одно имя сервера")

placeholder_like = ("replace", "changeme", "example.com", "example.org")
for key in ("SSTP_REMOTEHOST", "SSTP_PASSWORD", "POSTGRES_PASSWORD"):
    value = env.get(key, "").lower()
    if any(marker in value for marker in placeholder_like):
        fail(f"{key} всё ещё содержит шаблонное значение")

for key in ("SSTP_EGRESS_IMAGE", "TELEGRAM_PROXY_IMAGE", "CONTROL_PLANE_IMAGE"):
    value = env.get(key, "").lower()
    if any(marker in value for marker in ("replace", "changeme", "your-namespace", "<", ">")):
        fail(f"{key} всё ещё содержит шаблонное значение")

bot_token = env.get("TELEGRAM_BOT_TOKEN", "")
if bot_token and "replace" in bot_token.lower():
    fail("TELEGRAM_BOT_TOKEN задан шаблонным значением")

mtproto_secret = env.get("MTPROTO_SECRET_VALUE", "")
if not mtproto_secret:
    fail("MTPROTO_SECRET_VALUE обязателен")
if "replace" in mtproto_secret.lower():
    fail("MTPROTO_SECRET_VALUE задан шаблонным значением")

admin_token = env.get("CONTROL_PLANE_ADMIN_TOKEN", "")
if not admin_token:
    warn("CONTROL_PLANE_ADMIN_TOKEN не задан: /api/admin/* будет отключён")
elif len(admin_token) < 24:
    warn("CONTROL_PLANE_ADMIN_TOKEN короче 24 символов; для продового теста лучше длиннее")
warn("Шаблоны, миграции и generated-конфиги хранятся внутри образов и docker volume; структура репозитория на production-хосте не нужна")

print("Проверка env-переменных прошла успешно.")
PY

docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" config -q
echo "docker compose config проверен успешно."
