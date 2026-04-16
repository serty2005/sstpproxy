# Secrets

В этот каталог не нужно коммитить реальные секреты.

Ожидаемые пути для production-compose:

- `deploy/secrets/mtproto/secret` — текущий MTProto secret. Его нужно создать заранее.
- приватный REALITY key хранится не в репозитории, а в docker volume `reality-secrets`, который control-plane заполняет сам при первом bootstrap.

Каталог `deploy/secrets/mtproto` смонтирован в `control-plane` как `/srv/secrets/mtproto`.
