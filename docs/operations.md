# Эксплуатация

## Подготовка

1. Скопируйте `deploy/compose/.env.example` в `deploy/compose/.env`.
2. Создайте `deploy/secrets/mtproto/secret` и положите туда действительный MTProto secret.
3. Создайте каталог `deploy/secrets/reality`. Первый private key для REALITY появится там автоматически при bootstrap.
4. Проверьте, что каталог секретов доступен на запись контейнеру `control-plane`.

## Запуск стека

```bash
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env up -d --build
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

## Полезные каталоги

- `infra/xray/generated` — активный и архивные Xray config.
- `infra/mtg/generated` — активный и архивные MTProto config.
- `deploy/secrets/reality` — REALITY private key files.
- `deploy/secrets/mtproto` — MTProto secret file.
