#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
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

"${PYTHON_BIN}" - "${ENV_FILE}" "${REPO_ROOT}" <<'PY'
from __future__ import annotations

import re
import sys
from pathlib import Path

env_path = Path(sys.argv[1])
repo_root = Path(sys.argv[2])


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
    "SSTP_REMOTEHOST",
    "SSTP_USERNAME",
    "SSTP_PASSWORD",
    "SSTP_IMAGE_TAG",
    "SSTP_IMAGE_DIGEST",
    "PUBLIC_HOST",
    "XRAY_PORT",
    "MTG_PORT",
    "CONTROL_PLANE_PORT",
    "XRAY_REALITY_DEST",
    "XRAY_REALITY_SERVER_NAMES",
    "POSTGRES_DB",
    "POSTGRES_USER",
    "POSTGRES_PASSWORD",
    "XRAY_VERSION",
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

if not env["SSTP_IMAGE_DIGEST"].startswith("sha256:"):
    fail("SSTP_IMAGE_DIGEST должен начинаться с sha256:")

if not env["XRAY_VERSION"].startswith("v"):
    fail("XRAY_VERSION должен быть в формате release tag, например v26.3.27")

if not [part.strip() for part in env["XRAY_REALITY_SERVER_NAMES"].split(",") if part.strip()]:
    fail("XRAY_REALITY_SERVER_NAMES должен содержать хотя бы одно имя сервера")

placeholder_like = ("replace", "changeme", "example.com", "example.org")
for key in ("SSTP_REMOTEHOST", "SSTP_PASSWORD", "POSTGRES_PASSWORD"):
    value = env.get(key, "").lower()
    if any(marker in value for marker in placeholder_like):
        fail(f"{key} всё ещё содержит шаблонное значение")

bot_token = env.get("TELEGRAM_BOT_TOKEN", "")
if bot_token and "replace" in bot_token.lower():
    fail("TELEGRAM_BOT_TOKEN задан шаблонным значением")

admin_token = env.get("CONTROL_PLANE_ADMIN_TOKEN", "")
if not admin_token:
    warn("CONTROL_PLANE_ADMIN_TOKEN не задан: /api/admin/* будет отключён")
elif len(admin_token) < 24:
    warn("CONTROL_PLANE_ADMIN_TOKEN короче 24 символов; для продового теста лучше длиннее")

paths_must_exist = {
    "Xray server template": repo_root / "infra/xray/templates/config.json.tmpl",
    "Xray client template nekoray": repo_root / "infra/xray/templates/client/nekoray.json.tmpl",
    "Xray client template hiddify": repo_root / "infra/xray/templates/client/hiddify.json.tmpl",
    "Xray client template v2rayn": repo_root / "infra/xray/templates/client/v2rayn.json.tmpl",
    "MTProto template": repo_root / "infra/mtg/mtg.toml.tmpl",
    "Xray generated dir": repo_root / "infra/xray/generated",
    "MTProto generated dir": repo_root / "infra/mtg/generated",
    "Reality secrets dir": repo_root / "deploy/secrets/reality",
    "MTProto secrets dir": repo_root / "deploy/secrets/mtproto",
}

for label, path in paths_must_exist.items():
    if not path.exists():
        fail(f"отсутствует {label}: {path}")

mtproto_secret = repo_root / "deploy/secrets/mtproto/secret"
if not mtproto_secret.is_file():
    fail(f"не найден MTProto secret file: {mtproto_secret}")
if mtproto_secret.stat().st_size == 0:
    fail(f"MTProto secret file пуст: {mtproto_secret}")

reality_active_key = repo_root / "deploy/secrets/reality/active.key"
if not reality_active_key.exists():
    warn(f"файл {reality_active_key} пока отсутствует; при первом bootstrap control-plane может создать его сам")

for writable_path in (
    repo_root / "infra/xray/generated",
    repo_root / "infra/mtg/generated",
    repo_root / "deploy/secrets/reality",
):
    if not writable_path.is_dir():
        fail(f"ожидалась директория: {writable_path}")

digest_re = re.compile(r"^sha256:[0-9a-f]{64}$")
if not digest_re.match(env["SSTP_IMAGE_DIGEST"]):
    fail("SSTP_IMAGE_DIGEST должен содержать полный sha256 digest")

print("Проверка env, шаблонов и secret-файлов прошла успешно.")
PY

docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" config -q
echo "docker compose config проверен успешно."
