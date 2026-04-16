# Secrets

В этот каталог не нужно коммитить реальные секреты.

Production-compose больше не использует файлы из этого каталога напрямую.

- приватный REALITY key хранится в docker volume `reality-secrets`;
- MTProto secret хранится в docker volume `mtproto-secrets` и создаётся из переменной `MTPROTO_SECRET_VALUE`;
- если нужен локальный резервный экспорт секретов, держите его вне репозитория.
