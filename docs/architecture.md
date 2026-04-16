# Архитектура

## Контейнеры

- `sstp-egress` остаётся владельцем `ppp0`, policy routing и опубликованных портов.
- `xray-edge` и `mtg-edge` работают в `network_mode: "service:sstp-egress"`, поэтому не публикуют свои порты отдельно.
- `control-plane` находится в обычной внутренней сети `backend`, не живёт внутри SSTP namespace и не зависит от `ppp0` для своей административной работы.
- `docker-socket-proxy` даёт control-plane только минимальный набор Docker API для inspect/restart контейнера Xray.
- `postgres` используется как основное production-хранилище.

## Control-plane

Control-plane разбит на следующие слои:

- `internal/config` читает и валидирует env-конфигурацию.
- `internal/storage` содержит общий SQL-слой для Postgres и SQLite.
- `internal/service` реализует бизнес-логику: bootstrap keyset, пул shortId, пользователей, рендер/apply, MTProto metadata и audit.
- `internal/xray` отвечает за `xray x25519`, шаблоны, валидацию и атомарную запись.
- `internal/mtproto` отвечает за чтение secret file, invite link и рендер `mtg.toml`.
- `internal/httpapi` и `internal/telegram` дают административные интерфейсы поверх одного и того же фасада сервисов.

## REALITY модель

- На сервере держится один активный REALITY keyset.
- `privateKey` хранится только в docker volume `reality-secrets`, который монтируется в `control-plane` как `/srv/secrets/reality`.
- MTProto secret хранится в docker volume `mtproto-secrets`, который заполняет `control-plane-init` из `MTPROTO_SECRET_VALUE`.
- `publicKey` и путь до secret file хранятся в таблице `reality_keysets`.
- shortId живут в отдельной таблице `reality_short_ids`.
- При создании пользователя shortId выделяется из активного пула, а при revoke возвращается обратно в пул.
- При bootstrap, если активного keyset нет, control-plane сам вызывает `xray x25519`, пишет private key в файл и заполняет пул shortId.

## Ротация

- Ротация REALITY keyset не происходит автоматически на деплой.
- Отдельная административная операция доступна через `POST /api/admin/xray/keyset/rotate`.
- При ротации control-plane создаёт новый keyset, новый пул shortId, перевязывает активных пользователей на новые shortId и затем применяет новый Xray config.
- Если apply после ротации не проходит, база уже содержит новый keyset, поэтому администратор должен повторить `xray_apply` после устранения причины.

## Хранилище

Обязательные таблицы:

- `users`
- `reality_keysets`
- `reality_short_ids`
- `rendered_configs`
- `audit_events`

Дополнительно используется `mtproto_secrets`, потому что MTProto secret должен иметь metadata в БД, но фактическое значение берётся из secret file.

Практическая оговорка:

- Поле `users.reality_short_id_id` оставлено без физического foreign key, потому что модель одновременно требует и `users.reality_short_id_id`, и `reality_short_ids.assigned_user_id`, а такой цикл неудобно поддерживать одинаково в Postgres и SQLite миграциях.
- Целостность этой связи удерживается сервисным слоем и уникальными индексами.
