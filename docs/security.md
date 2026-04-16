# Безопасность

## Что уже сделано

- REALITY `privateKey` не хранится в PostgreSQL.
- MTProto secret не хранится в git и записывается init-контейнером в volume `mtproto-secrets`.
- `deploy/secrets` закрыт `.gitignore`, а REALITY private key вынесен в docker volume `reality-secrets`.
- `control-plane`, `xray-edge` и `mtg-edge` используют `read_only` root filesystem там, где это возможно.
- Для `control-plane`, `xray-edge` и `mtg-edge` включён `no-new-privileges`.
- `control-plane`, `xray-edge` и `mtg-edge` работают с `cap_drop: [ALL]`.
- Docker API доступен только через `docker-socket-proxy` и только внутри backend-сети.
- `postgres` не публикуется наружу.
- `control-plane` публикуется только на `127.0.0.1`.
- Все изменения пользователей, рендеров и административных операций пишутся в `audit_events`.

## Что нужно помнить в эксплуатации

- следите, чтобы docker volume `mtproto-secrets` и `reality-secrets` были доступны только доверенным администраторам;
- после ротации REALITY keyset нужно перевыпустить пользовательские профили;
- логи control-plane не должны уходить в публичные системы без фильтрации, если в будущем туда добавятся дополнительные поля;
- если меняется `MTPROTO_SECRET_VALUE`, control-plane на следующем bootstrap или следующем рендере зафиксирует новую metadata-запись в БД.

## Ротация секретов

REALITY:

1. Выполнить `POST /api/admin/xray/keyset/rotate`.
2. Перевыпустить клиентские URI и JSON-профили.
3. Убедиться, что новый `config.json` применён и `xray-edge` healthy.

MTProto:

1. Обновить `MTPROTO_SECRET_VALUE` в `.env` и повторно выполнить `docker compose up -d`.
2. Перезапустить `control-plane` или выполнить операцию, которая заново отрендерит MTProto config.
3. Раздать новую `tg://proxy?...` ссылку пользователям.
