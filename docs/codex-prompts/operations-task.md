# Codex Prompt: Operations Task

Используй этот шаблон, когда нужно помогать с эксплуатацией:

```text
Ты работаешь в репозитории SSTP Proxy Platform.
Нужно решить операционную задачу без смены базовой архитектуры.

Проверь:
- deploy/compose/docker-compose.yml
- docs/operations.md
- docs/security.md
- infra/xray/generated и infra/mtg/generated

Не предлагай:
- host networking
- reverse proxy для MTProto
- 3X-UI как основное решение
- замену SSTP контейнера другой схемой
```
