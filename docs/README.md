# Документация ruto

Набор документов для быстрого погружения в проект.

| Документ | О чём |
| --- | --- |
| [overview.md](overview.md) | Что такое ruto, два бэкенда, ключевые понятия, карта репозитория |
| [architecture.md](architecture.md) | Слои, поток запроса, механизм snapshot, конфигурационный pipeline |
| [sso.md](sso.md) | Вход через шлюз: OpenID Connect провайдер, куки, обновление сессии, брокер для фронтов (вход стандартными SDK) |
| [code-style.md](code-style.md) | Соглашения по коду и устоявшиеся паттерны |
| [onboarding.md](onboarding.md) | Локальный запуск и пошаговые рецепты (добавить API / сущность / миграцию) |
| [gateway-grpc.md](gateway-grpc.md) | Подключение внешних клиентов к gateway по gRPC |
| [monitoring.md](monitoring.md) | Графики трафика и логи в админке (Prometheus / Loki / Graylog) |
| [pulse.md](pulse.md) | Интеграция с pulse: манифест и `/routes` на служебном порту core |
| [mcp-server.md](mcp-server.md) | Встроенный MCP-сервер для AI-агентов: ключи, OAuth, маскирование секретов, заявки |

`api.swagger.json` генерируется `make generate-proto`, `grafana/` — дашборд метрик шлюза (см. monitoring.md).
