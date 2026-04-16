# Эксплуатация

## Подготовка

1. Скопируйте `deploy/compose/.env.example` в `deploy/compose/.env`.
2. Создайте `deploy/secrets/mtproto/secret` и положите туда действительный MTProto secret.
3. Укажите в `deploy/compose/.env` имена опубликованных образов для `SSTP_EGRESS_IMAGE`, `TELEGRAM_PROXY_IMAGE` и `CONTROL_PLANE_IMAGE`.
4. Первый private key для REALITY появится автоматически в docker volume `reality-secrets` при bootstrap.

## Запуск стека

```bash
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env pull
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env up -d
```

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

## Полезные данные и секреты

- `deploy/secrets/mtproto/secret` — MTProto secret file на хосте.
- volume `reality-secrets` — REALITY private key files.
- volume `xray-generated` — активный и архивные Xray config.
- volume `mtg-generated` — активный и архивные MTProto config.
