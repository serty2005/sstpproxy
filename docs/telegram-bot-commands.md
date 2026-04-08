# Telegram Bot Commands

Поддерживаемые команды:

- `/start` — краткая справка.
- `/help` — список команд.
- `/users` — список пользователей с ID, UUID, shortId и статусом.
- `/user_add <display_name>` — создать пользователя.
- `/user_revoke <user_id_or_uuid>` — деактивировать пользователя и освободить shortId.
- `/user_link <user_id_or_uuid>` — вернуть VLESS URI.
- `/user_profile <user_id_or_uuid> <nekoray|hiddify|v2rayn>` — вернуть JSON-профиль.
- `/xray_render` — отрендерить Xray config и сохранить metadata.
- `/xray_apply` — провалидировать и применить Xray config.
- `/mtproto` — вернуть текущую `tg://proxy?...` ссылку.
- `/health` — показать сокращённый readiness-статус.

Ограничения:

- доступ есть только у Telegram user ID из `TELEGRAM_ADMIN_IDS`;
- ошибки в чат отправляются в сжатом виде без stack trace;
- каждое административное действие проходит через `audit_events`.
