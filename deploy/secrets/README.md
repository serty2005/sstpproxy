# Secrets

В этот каталог не нужно коммитить реальные секреты.

Ожидаемые пути для production-compose:

- `deploy/secrets/reality/active.key` — приватный REALITY key, который control-plane создаёт сам при первом bootstrap.
- `deploy/secrets/mtproto/secret` — текущий MTProto secret. Его нужно создать заранее.

Каталог смонтирован в `control-plane` как `/srv/secrets`.
