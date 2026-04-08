# Codex Prompt: Control-plane Feature

Используй этот шаблон, когда нужно расширять Go control-plane:

```text
Ты работаешь в репозитории SSTP Proxy Platform.
Нужно доработать только services/control-plane и связанные шаблоны/доки.

Требования:
- не ломать network_mode service:sstp-egress
- не переносить бизнес-логику в shell
- privateKey REALITY не класть в БД
- новые административные действия аудировать через audit_events
- если меняются пользовательские данные Xray, учитывать render/apply цикл

Перед изменениями:
1. Прочитай docs/architecture.md и docs/security.md.
2. Проверь migrations и доменные интерфейсы.
3. Если добавляешь новую сущность, зафиксируй её в docs/architecture.md.
```
