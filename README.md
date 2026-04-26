# SSTP Proxy Platform

Production-ready каркас сервиса доступа через SSTP-egress, Xray REALITY, MTProto и отдельный Go control-plane.

Основные точки запуска:

- [docker-compose.yml](/c:/safe/repos/sstpproxy/docker-compose.yml) для локальной сборки и тегирования образов
- [.env.example](/c:/safe/repos/sstpproxy/.env.example) для локальной сборки
- [deploy/compose/docker-compose.yml](/c:/safe/repos/sstpproxy/deploy/compose/docker-compose.yml) для production runtime
- [deploy/compose/.env.example](/c:/safe/repos/sstpproxy/deploy/compose/.env.example) для production runtime
- [main.go](/c:/safe/repos/sstpproxy/services/control-plane/cmd/control-plane/main.go)

Ключевые свойства текущей реализации:

- `sstp-egress` остаётся владельцем `ppp0`, policy routing и опубликованных портов.
- `xray-edge` и `mtg-edge` вынесены в отдельные контейнеры и используют `network_mode: "service:sstp-egress"`.
- `control-plane` работает как отдельный Go-сервис с PostgreSQL, миграциями, шаблонами, Telegram-ботом, audit trail и безопасным apply Xray-конфига.
- шаблоны Xray, MTProto и миграции упакованы в `control-plane` образ, поэтому production-compose не зависит от структуры репозитория на хосте.
- приватный REALITY key хранится в docker volume `reality-secrets`, а MTProto secret засеивается init-контейнером в volume `mtproto-secrets`.
- shortId управляются отдельным пулом и назначаются пользователям из control-plane.

Локальная сборка и публикация образов:

1. Скопируйте [.env.example](/c:/safe/repos/sstpproxy/.env.example) в корневой `.env` и укажите финальные теги Docker Hub для `SSTP_EGRESS_IMAGE`, `TELEGRAM_PROXY_IMAGE` и `CONTROL_PLANE_IMAGE`.
2. Соберите кастомные образы локально:

```bash
docker compose build
```

3. Опубликуйте их в реестр:

```bash
docker compose push
```

Быстрый запуск production-стека:

1. Скопируйте [deploy/compose/.env.example](/c:/safe/repos/sstpproxy/deploy/compose/.env.example) в `deploy/compose/.env` и заполните runtime-переменные, включая имена опубликованных образов, `MTPROTO_SECRET_VALUE`, `XRAY_VERSION` и `XRAY_IMAGE_TAG`.
2. Перенесите на production-хост только `docker-compose.yml`, `.env` и при необходимости служебные скрипты из `deploy/compose`.
3. Подтяните образы и запустите стек:

```bash
docker compose -f /root/sstproxy/docker-compose.yml --env-file /root/sstproxy/.env pull
docker compose -f /root/sstproxy/docker-compose.yml --env-file /root/sstproxy/.env up -d
```

4. Проверьте готовность control-plane:

```bash
curl http://127.0.0.1:8080/readyz
```

5. После старта добавьте пользователя через Telegram-бота или внутренний admin API и выполните `/xray_apply`, если меняли состав пользователей вручную.

Практическая оговорка по Xray:

- `XRAY_VERSION` используется для скачивания бинаря Xray в образ `control-plane` и должен быть с префиксом `v`, например `v26.3.27`.
- `XRAY_IMAGE_TAG` используется для контейнера `ghcr.io/xtls/xray-core` и должен быть без префикса `v`, например `26.3.27`.

Публикуемые порты production-стека:

- `${XRAY_PORT}/tcp` на `sstp-egress` для входящих Xray REALITY-подключений. По умолчанию `4443/tcp`.
- `${MTG_PORT}/tcp` на `sstp-egress` для входящих MTProto-подключений. По умолчанию `4430/tcp`.
- `127.0.0.1:${CONTROL_PLANE_PORT}/tcp` на `control-plane` только для локального администрирования. По умолчанию `127.0.0.1:8080/tcp`.
- `postgres`, `docker-socket-proxy` и `telegram-proxy` наружу не публикуются.

Быстрый старт, эксплуатационные шаги и модель безопасности описаны в документации:

- [architecture.md](/c:/safe/repos/sstpproxy/docs/architecture.md)
- [operations.md](/c:/safe/repos/sstpproxy/docs/operations.md)
- [security.md](/c:/safe/repos/sstpproxy/docs/security.md)
- [telegram-bot-commands.md](/c:/safe/repos/sstpproxy/docs/telegram-bot-commands.md)
