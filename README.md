# Team Tasks

REST API сервис для управления задачами в командах. Go · MySQL 8.4 · Redis 7 · Docker Compose.

## Возможности

### Основной функционал

1. **Регистрация и аутентификация** — JWT HS256, bcrypt-хэш паролей
2. **Управление командами**
   - Создание команды (создатель становится owner)
   - Список команд, в которых состоит пользователь
   - Приглашение пользователя (только owner / admin)
   - Ролевая модель: `owner` / `admin` / `member`
3. **Управление задачами**
   - Создание задачи (только член команды)
   - Список задач с фильтрами по команде, статусу, исполнителю + пагинация
   - Обновление задачи (только член команды)
   - История изменений: каждое изменённое поле — отдельная запись с audit-trail
4. **Комментарии** — добавление и просмотр комментариев к задаче (только члены команды)
5. **Аналитика** (сложные SQL-запросы)
   - Сводка по командам: JOIN + агрегация
   - Топ-3 контрибьютора в команде: оконная функция `RANK()`
   - Orphan-задачи: задачи, где исполнитель больше не член команды (подзапрос)

### Нефункциональные требования

- Кеш списка задач в Redis (cache-aside, TTL 5 мин, инвалидация по паттерну при изменениях)
- Prometheus метрики (`GET /metrics`): `http_requests_total`, `http_request_duration_seconds`
- Rate limiting: 100 req/min per user (скользящее окно, ключ — userID или IP)
- Circuit breaker для внешнего email-сервиса (sony/gobreaker, открывается после 3 сбоев подряд)
- Graceful shutdown (SIGINT/SIGTERM, таймаут из конфига)
- Интеграционные тесты с реальными MySQL и Redis через testcontainers-go

## API

Полная спецификация: [`api/v1/openapi.yaml`](api/v1/openapi.yaml) (OpenAPI 3.0).

Посмотреть в браузере:

```bash
make docs          # npx @redocly/cli preview-docs (требует Node.js)
```

Или вставить `api/v1/openapi.yaml` в [editor.swagger.io](https://editor.swagger.io).

## Архитектура

```
cmd/teamtasks/      — точка входа, запуск сервера
internal/
  app/              — сборка всех зависимостей, роутер
  config/           — конфиг из YAML и переменных окружения
  domain/           — модели данных и типы ошибок
  handler/          — HTTP-хендлеры (chi + oapi-codegen)
  middleware/       — JWT, rate limiting, метрики запросов
  metrics/          — Prometheus-счётчики
  service/
    auth/           — регистрация и вход (JWT)
    team/           — команды и приглашения
    task/           — задачи, история изменений, Redis-кеш
    comment/        — комментарии к задачам
    analytics/      — аналитические отчёты
  storage/
    mysql/          — реализация работы с БД
    redis/          — кеш списка задач
  circuitbreaker/   — защита вызовов внешнего email-сервиса
api/                — OpenAPI-спецификация и сгенерированный код
migrations/         — SQL-миграции (таблицы и индексы)
tests/              — интеграционные тесты с реальными MySQL и Redis
```

## Схема БД

> Диаграмма генерируется из [`migrations/*.up.sql`](migrations/) командой `make schema` — не редактировать вручную.

<!-- schema:start -->
```mermaid
erDiagram
    users {
        binary id PK
        varchar email UK
        varchar name
        varchar password
        datetime created_at
    }
    teams {
        binary id PK
        varchar name
        binary created_by FK
        datetime created_at
    }
    team_members {
        binary team_id PK,FK
        binary user_id PK,FK
        enum role
        datetime joined_at
    }
    tasks {
        binary id PK
        binary team_id FK
        binary created_by FK
        binary assignee_id FK
        varchar title
        text description
        enum status
        enum priority
        date due_date
        datetime created_at
        datetime updated_at
    }
    task_history {
        bigint id PK
        binary task_id FK
        binary changed_by FK
        varchar field
        text old_value
        text new_value
        datetime changed_at
    }
    task_comments {
        bigint id PK
        binary task_id FK
        binary user_id FK
        text body
        datetime created_at
    }
    users ||--o{ teams : "created_by"
    teams ||--o{ team_members : "team_id"
    users ||--o{ team_members : "user_id"
    teams ||--o{ tasks : "team_id"
    users ||--o{ tasks : "created_by"
    users ||--o{ tasks : "assignee_id"
    tasks ||--o{ task_history : "task_id"
    users ||--o{ task_history : "changed_by"
    tasks ||--o{ task_comments : "task_id"
    users ||--o{ task_comments : "user_id"
```
<!-- schema:end -->

## Быстрый старт

```bash
# 1. Клонировать
git clone https://github.com/lkmavi/team-tasks
cd team-tasks

# 2. Переменные окружения
cp .env.example .env
# Задать: MYSQL_ROOT_PASSWORD, MYSQL_USER, MYSQL_PASSWORD, AUTH_JWT_SECRET

# 3. Запуск
docker compose up
```

API доступен на `http://localhost:8080/api/v1`.

### Пример: регистрация, вход, создание команды и задачи

```bash
# Регистрация
curl -s -X POST http://localhost:8080/api/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","name":"Alice","password":"secret123"}'

# Вход
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret123"}' | jq -r .token)

# Создать команду
TEAM_ID=$(curl -s -X POST http://localhost:8080/api/v1/teams \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Backend Team"}' | jq -r .id)

# Создать задачу
curl -s -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"team_id\":\"$TEAM_ID\",\"title\":\"Fix the bug\",\"priority\":\"high\"}"

# Список задач с фильтром
curl -s "http://localhost:8080/api/v1/tasks?team_id=$TEAM_ID&status=todo" \
  -H "Authorization: Bearer $TOKEN"
```

## Тестирование API вручную

### Bruno-коллекция

[Bruno](https://www.usebruno.com/) — open-source API-клиент (альтернатива Postman). Коллекция находится в папке [`bruno/`](bruno/).

**Установка Bruno:**

```bash
# macOS
brew install bruno

# Windows / Linux — скачать с https://www.usebruno.com/downloads
```

**Открыть коллекцию:**

1. В меню выбрать **File → Open Collection**
2. Указать папку `bruno/` в корне проекта
3. В правом верхнем углу выбрать окружение **local** — **обязательно до отправки любого запроса**, иначе `{{baseUrl}}` и `{{token}}` не будут подставлены

**Переменные окружения** (`bruno/environments/local.bru`):

| Переменная      | Значение по умолчанию          | Как заполняется                                      |
|-----------------|-------------------------------|-------------------------------------------------------|
| `baseUrl`       | `http://localhost:8080/api/v1` | задана в файле окружения                              |
| `token`         | *(пусто)*                      | после `auth/Login` — скрипт сохраняет автоматически  |
| `teamId`        | *(пусто)*                      | после `teams/Create Team` — скрипт сохраняет         |
| `taskId`        | *(пусто)*                      | после `tasks/Create Task` — скрипт сохраняет         |
| `invitedUserId` | *(пусто)*                      | вставить вручную (UUID нужного пользователя)          |

Если переменная не сохранилась автоматически — откройте **Environments → local → Edit** и вставьте значение из тела ответа вручную.

**Типичный сценарий работы:**

```
0. Выбрать окружение local (правый верхний угол)
1. auth/Register        — зарегистрировать пользователя
2. auth/Login           — войти; скрипт запишет {{token}} в окружение
3. teams/Create Team    — создать команду; скрипт запишет {{teamId}}
4. teams/Invite Member  — пригласить другого пользователя (вставить {{invitedUserId}} вручную)
5. tasks/Create Task    — создать задачу; скрипт запишет {{taskId}}
6. tasks/Update Task    — изменить статус / приоритет
7. tasks/Add Comment    — добавить комментарий
8. tasks/Get History    — посмотреть историю изменений
9. analytics/*          — аналитические отчёты
```

---

### Заполнение тестовыми данными

Сидер обращается к запущенному API и создаёт набор реалистичных данных:

```bash
# Запускать при поднятом docker compose up
go run ./scripts/seed

# Другой адрес
go run ./scripts/seed --base-url http://localhost:8080/api/v1
```

Сидер создаёт:

| Объект   | Количество | Детали                                                              |
|----------|-----------|----------------------------------------------------------------------|
| Пользователи | 3     | alice, bob, carol (пароль `secret123`)                             |
| Команды  | 2         | Backend Squad (owner: alice), Frontend Guild (owner: bob)           |
| Задачи   | 6         | статусы: todo / in_progress / done; приоритеты: low / medium / high |
| Комментарии | 5      | распределены по задачам                                             |

По завершению сидер печатает токены всех трёх пользователей и UUID всех созданных объектов. Их можно вставить в переменные Bruno-окружения через **Environments → local → Edit**.

## Сервисы

| Сервис     | Адрес                  | Описание          |
|------------|------------------------|-------------------|
| API        | http://localhost:8080  | REST API          |
| Prometheus | http://localhost:9090  | Метрики           |
| Adminer    | http://localhost:8090  | UI для MySQL      |
| MySQL      | localhost:3306         | База данных       |
| Redis      | localhost:6379         | Кеш               |

### Prometheus

Метрики доступны на `GET /metrics`. После нескольких запросов к API:

```promql
# RPS по маршрутам
rate(http_requests_total[1m])

# P99 latency
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
```

## Разработка

```bash
# Кодогенерация (OpenAPI)
go generate ./...

# Юнит-тесты
make test

# Интеграционные тесты API (требуется Docker)
make test-integration

# Тесты с покрытием (≥85%)
make test-cover

# Линтер
make lint

# Проверка перед релизом
bash scripts/pre-release-check.sh
```

## Миграции

Применяются автоматически сервисом `migrate` при старте compose.

```bash
# Версия
docker compose run --rm migrate -path=/migrations \
  -database "mysql://${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(mysql:3306)/team_tasks?multiStatements=true&loc=UTC" \
  version

# Откат
docker compose run --rm migrate -path=/migrations \
  -database "mysql://${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(mysql:3306)/team_tasks?multiStatements=true&loc=UTC" \
  down 1
```

## Зависимости

| Пакет | Назначение |
|-------|-----------|
| `github.com/go-chi/chi/v5` | HTTP-роутер |
| `github.com/oapi-codegen/oapi-codegen/v2` | Кодогенерация из OpenAPI |
| `github.com/golang-jwt/jwt/v5` | JWT |
| `github.com/jmoiron/sqlx` | SQL |
| `github.com/redis/go-redis/v9` | Redis |
| `github.com/google/uuid` | UUID v7 (time-ordered) |
| `github.com/ilyakaznacheev/cleanenv` | Конфигурация |
| `github.com/prometheus/client_golang` | Prometheus |
| `github.com/sony/gobreaker` | Circuit breaker |
| `golang.org/x/time/rate` | Rate limiting |
| `golang.org/x/crypto/bcrypt` | Хеширование паролей |
| `github.com/testcontainers/testcontainers-go` | Интеграционные тесты |
| `github.com/golang-migrate/migrate/v4` | Миграции |
| `pkg/slogx` | Структурированный логгер (вендорированный) |
