# Эксплуатация

## Подготовка

1. Скопируйте `deploy/compose/.env.example` в `deploy/compose/.env`.
2. Укажите в `deploy/compose/.env` имена опубликованных образов для `SSTP_EGRESS_IMAGE`, `TELEGRAM_PROXY_IMAGE` и `CONTROL_PLANE_IMAGE`.
3. Заполните `MTPROTO_SECRET_VALUE` реальным MTProto secret. Для `mtg` v2 нужен FakeTLS secret: base64 или hex-строка с префиксом `ee`.
4. Укажите `XRAY_VERSION` в формате GitHub release tag, например `v26.3.27`, и `XRAY_IMAGE_TAG` в формате container tag, например `26.3.27`.
5. При первом bootstrap `control-plane-init` сам создаст docker volume `reality-secrets`, `mtproto-secrets`, `xray-generated` и `mtg-generated`.
6. На production-хосте не нужна структура каталогов репозитория: шаблоны и миграции уже лежат внутри `control-plane` образа.

## Запуск стека

```bash
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env pull
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env up -d
```

Секрет можно сгенерировать командой:

```bash
docker run --rm nineseconds/mtg:2.2.8 generate-secret --hex <front-domain>
```

`<front-domain>` должен быть выбран осознанно как домен для FakeTLS/domain fronting. После изменения секрета пересоздайте стек, чтобы `control-plane-init` заново записал docker volume `mtproto-secrets`.

## Проверка состояния

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

`readyz` дополнительно проверяет:

- подключение к БД;
- доступность Docker API через proxy;
- наличие активного REALITY keyset;
- наличие активных конфигов Xray и MTProto на диске.

## Создание пользователя

Через Telegram:

```text
/user_add Имя Пользователя
/user_link <user_id_or_uuid>
/user_profile <user_id_or_uuid> nekoray
```

`/user_add` и `/user_revoke` в Telegram-боте автоматически применяют новый Xray config, поэтому отдельный `/xray_apply` после этих команд обычно не нужен.

Через HTTP API:

```bash
curl -X POST http://127.0.0.1:8080/api/admin/users \
  -H "Content-Type: application/json" \
  -H "X-Admin-Actor: local-admin" \
  -d '{"display_name":"Test User"}'
```

## Рендер и применение Xray config

Рендер:

```bash
curl -X POST http://127.0.0.1:8080/api/admin/xray/render -H "X-Admin-Actor: local-admin"
```

Применение:

```bash
curl -X POST http://127.0.0.1:8080/api/admin/xray/apply -H "X-Admin-Actor: local-admin"
```

## Ротация REALITY keyset

```bash
curl -X POST http://127.0.0.1:8080/api/admin/xray/keyset/rotate -H "X-Admin-Actor: local-admin"
```

После ротации клиентские URI и JSON-профили нужно перевыпустить, потому что у пользователя меняются `pbk` и `sid`.

## Порты, которые публикуются наружу

- `${XRAY_PORT}/tcp` -> `sstp-egress` -> `xray-edge`. По умолчанию `4443/tcp`.
- `${MTG_PORT}/tcp` -> `sstp-egress` -> `mtg-edge`. По умолчанию `4430/tcp`.
- `127.0.0.1:${CONTROL_PLANE_PORT}/tcp` -> `control-plane`. По умолчанию `127.0.0.1:8080/tcp`.
- `postgres`, `docker-socket-proxy` и `telegram-proxy` остаются внутренними сервисами без внешнего published port.

## Диагностика MTProto

`telegram-proxy` — это внутренний HTTP CONNECT-прокси для Telegram Bot API, а не MTProto-прокси. Его строки вида `CONNECT api.telegram.org:443` относятся к healthcheck или работе бота.

MTProto обслуживает контейнер `mtg-edge`. Проверяйте его отдельно:

```bash
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env logs mtg-edge
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env exec mtg-edge /mtg access /etc/mtg/mtg.toml
```

## Полезные данные и секреты

- volume `reality-secrets` — REALITY private key files.
- volume `mtproto-secrets` — MTProto secret file, созданный из `MTPROTO_SECRET_VALUE`.
- volume `xray-generated` — активный и архивные Xray config.
- volume `mtg-generated` — активный и архивные MTProto config.
