# bachata-reengage

Go-сервис повторного вовлечения клиентов через чат Bachata. Периодически опрашивает API, обнаруживает диалоги, в которых клиент не получил ответа, и фиксирует моменты отправки напоминаний.

## Логика напоминаний

1. **Первое напоминание** — в диалоге тишина ≥ 3 дней, телефон не получен → состояние `waiting_second`.
2. **Второе напоминание** — прошло ≥ 4 дней после первого (≈ день 7), телефон по-прежнему не получен → состояние `completed`.
3. **Отмена** — между первым и вторым напоминанием появился телефон → состояние `phone_received`, второе напоминание не отправляется.

## Структура

```
cmd/reengage/       — точка входа
internal/app/       — сборка зависимостей, запуск scheduler + metrics server
internal/config/    — конфигурация из переменных окружения
internal/api/       — HTTP-клиент Bachata API
internal/reminders/ — бизнес-логика напоминаний
internal/store/     — PostgreSQL-хранилище состояний
internal/scheduler/ — цикл периодического выполнения задач
internal/tasks/     — задача sync (запускает reminders.Service)
internal/metrics/   — Prometheus-метрики
configs/            — .env.example
```

## Переменные окружения

| Переменная        | Обязательная | По умолчанию                         | Описание                            |
|-------------------|:------------:|--------------------------------------|-------------------------------------|
| `API_TOKEN`       | ✓            | —                                    | Токен Bachata API                   |
| `DATABASE_URL`    | ✓            | —                                    | PostgreSQL DSN                      |
| `API_BASE_URL`    |              | `https://lk.bachata.tech/json/v1.0`  | Базовый URL API                     |
| `POLL_INTERVAL`   |              | `1m`                                 | Интервал опроса                     |
| `TASK_TIMEOUT`    |              | `30s`                                | Таймаут одного запуска задачи       |
| `REQUEST_TIMEOUT` |              | `15s`                                | Таймаут HTTP-запроса к API          |
| `LOOKBACK_WINDOW`        |              | `192h` (8 дней)             | Глубина поиска сообщений                |
| `FIRST_REMINDER_DELAY`  |              | `72h` (3 дня)               | Задержка перед первым напоминанием      |
| `SECOND_REMINDER_DELAY` |              | `96h` (4 дня после первого) | Задержка перед вторым напоминанием      |
| `METRICS_ADDR`    |              | `:8080`                              | Адрес HTTP-сервера метрик           |
| `DRY_RUN`         |              | `false`                              | Логировать вместо отправки          |
| `TEST_DIALOG_IDS` |              | —                                    | Обрабатывать только эти диалоги (через запятую) |

## Запуск

### Docker Compose (рекомендуется)

```bash
cp configs/.env.example configs/.env
# укажите API_TOKEN и OPERATOR_LOGIN в configs/.env

docker compose up -d
```

Поднимает PostgreSQL и сервис. Данные БД сохраняются в volume `postgres_data`.

### Локально (без Docker)

```bash
cp configs/.env.example configs/.env
# укажите API_TOKEN, OPERATOR_LOGIN и DATABASE_URL в configs/.env

docker compose up -d postgres   # только БД

export $(grep -v '^#' configs/.env | xargs)
go run ./cmd/reengage
```

### Dry-run (без реальной отправки)

Установите `DRY_RUN=true` в `configs/.env` — сервис будет логировать сообщения, но не отправлять их.

### Тест на одном диалоге

Установите `TEST_DIALOG_ID=<id>` — планировщик работает штатно, но на каждом тике обрабатывается только указанный диалог. Удобно комбинировать с `DRY_RUN=true`:

```bash
TEST_DIALOG_IDS=24562,24563 DRY_RUN=true go run ./cmd/reengage
```

Метрики Prometheus доступны на `http://localhost:8080/metrics`.

## Проверка

```bash
go test ./...
go vet ./...
go build ./...
```
