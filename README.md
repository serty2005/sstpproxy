# SSTP Proxy Platform

Production-ready каркас сервиса доступа через SSTP-egress, Xray REALITY, MTProto и отдельный Go control-plane.

Основная точка запуска:

- [docker-compose.yml](/c:/safe/repos/sstpproxy/deploy/compose/docker-compose.yml)
- [.env.example](/c:/safe/repos/sstpproxy/deploy/compose/.env.example)
- [main.go](/c:/safe/repos/sstpproxy/services/control-plane/cmd/control-plane/main.go)

Ключевые свойства текущей реализации:

- `sstp-egress` остаётся владельцем `ppp0`, policy routing и опубликованных портов.
- `xray-edge` и `mtg-edge` вынесены в отдельные контейнеры и используют `network_mode: "service:sstp-egress"`.
- `control-plane` работает как отдельный Go-сервис с PostgreSQL, миграциями, шаблонами, Telegram-ботом, audit trail и безопасным apply Xray-конфига.
- приватный REALITY key хранится в secret file, а в PostgreSQL лежит только metadata и публичный ключ.
- shortId управляются отдельным пулом и назначаются пользователям из control-plane.

Быстрый запуск:

1. Скопируйте [deploy/compose/.env.example](/c:/safe/repos/sstpproxy/deploy/compose/.env.example) в `deploy/compose/.env` и заполните значения окружения.
2. Подготовьте каталог `deploy/secrets/mtproto` и положите в `deploy/secrets/mtproto/secret` действительный MTProto secret.
3. Убедитесь, что `deploy/secrets/reality` существует и доступен на запись для `control-plane`.
4. Запустите стек:

```bash
docker compose -f deploy/compose/docker-compose.yml --env-file deploy/compose/.env up -d --build
```

5. Проверьте готовность control-plane:

```bash
curl http://127.0.0.1:8080/readyz
```

6. После старта добавьте пользователя через Telegram-бота или внутренний admin API и выполните `/xray_apply`, если меняли состав пользователей вручную.

Быстрый старт, эксплуатационные шаги и модель безопасности описаны в документации:

- [architecture.md](/c:/safe/repos/sstpproxy/docs/architecture.md)
- [operations.md](/c:/safe/repos/sstpproxy/docs/operations.md)
- [security.md](/c:/safe/repos/sstpproxy/docs/security.md)
- [telegram-bot-commands.md](/c:/safe/repos/sstpproxy/docs/telegram-bot-commands.md)
