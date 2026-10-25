# taskq — очередь фоновых задач на Go и Postgres

**Дата:** 2026-10-25
**Статус:** дизайн согласован в чате, ждёт ревью этой спеки
**Репозиторий:** `github.com/alians84/taskq` (публичный, GitHub)

---

## 1. Зачем этот проект

**Цель:** перейти с Junior на Middle в Go и получить проект для портфолио, который можно защитить на собеседовании.

**Формат работы:**
- каркас, инфраструктура и рутину (Docker, Makefile, CI, миграции, конфиг, роутер) пишет Claude;
- доменную логику, работу с БД, ядро конкурентности и тесты пишет автор, Claude их ревьюит.

**Бюджет:** 6–10 часов в неделю, около 4 недель (30–40 часов).

**Критерии успеха:**
1. Chaos-демо проходит: 3 воркера, 500 задач, во время работы один воркер убит через `docker kill`, и все задачи всё равно дошли до финального статуса (`succeeded` или `dead`).
2. В CI зелёные линтер, `go test -race ./...` и интеграционные тесты.
3. Автор пишет без подсказок и объясняет своими словами: горутины, каналы, `select`, `sync`, `context`, обёртки ошибок, интерфейсы на стороне потребителя, табличные тесты и fake-реализации, `net/http`, транзакции в Postgres, graceful shutdown.
4. Автор отвечает на вопросы с собеседования, которые накопились по итогам каждой недели (раздел 13).

## 2. Объём

### Входит в MVP
- Один бинарник `taskq` с подкомандами `api`, `worker`, `migrate`.
- HTTP API: создать задачу, получить задачу, получить список с фильтром, вручную перезапустить «мёртвую» задачу; `/healthz`, `/readyz`, `/metrics`.
- Типы задач: `webhook` (HTTP POST на URL) и `sleep` (отладка и chaos).
- Отложенный запуск (`run_at`), повторы с экспоненциальной задержкой и jitter, статус `dead` после исчерпания попыток, постоянные ошибки (Permanent).
- Таймаут выполнения через `context`, аренда (lease) с подбором «осиротевших» задач, fencing при записи результата.
- Идемпотентное создание по `idempotency_key`.
- Graceful shutdown для `api` и `worker`.
- Логи `slog` в JSON, метрики Prometheus (включая собственный `Collector`).
- Docker Compose: postgres, migrate, api, worker (масштабируемый), prometheus.
- Chaos-демо и README с разделом «Почему так».

### Не входит (YAGNI)
UI, авторизация, приоритеты и несколько очередей, cron-задачи, Redis или Kafka, gRPC, Kubernetes, Grafana, продление аренды (heartbeat), `LISTEN/NOTIFY`.

### Этап 2 (по желанию, после MVP)
Heartbeat-продление аренды, `LISTEN/NOTIFY` вместо опроса, приоритеты и очереди, cron-задачи, gRPC API, дашборд Grafana.

## 3. Окружение и стек

| Что | Решение |
|---|---|
| Go | 1.27 (`go 1.27` в `go.mod`), локально 1.27.3 |
| Модуль | `github.com/alians84/taskq` |
| HTTP | только `net/http` (паттерны роутинга `GET /jobs/{id}`), без chi |
| Логи | `log/slog`, JSON |
| Postgres | 18, драйвер `github.com/jackc/pgx/v5` (`pgxpool`) |
| Миграции | `github.com/pressly/goose/v3`, SQL встроен через `embed`, применяется подкомандой `taskq migrate up` |
| Метрики | `github.com/prometheus/client_golang` |
| Интеграционные тесты | `github.com/testcontainers/testcontainers-go` (модуль postgres) |
| Линтер | golangci-lint v2, закреплён в проекте через директиву `tool` в `go.mod` и запускается как `go tool golangci-lint run`. Глобальный golangci-lint v1 не трогаем |
| Моки | только написанные руками fake-реализации, без mockgen |
| CI | GitHub Actions |

Других внешних зависимостей не добавляем. Если без новой зависимости не обойтись, её сначала обсуждаем.

## 4. Архитектура

```
taskq/
├── cmd/taskq/main.go       подкоманды api | worker | migrate, сборка зависимостей     [Claude]
├── internal/
│   ├── config/             env → структура Config, валидация                        [Claude]
│   ├── job/                Job, Status, доменные ошибки, PermanentError,
│   │                       Backoff, NextStep                                         [автор]
│   ├── store/              реализация на Postgres: Create, Get, List, Claim,
│   │                       Complete, Fail, Retry, CountByStatus                      [автор]
│   ├── api/                роутер, middleware (request-id, логирование, recover)    [Claude]
│   │                       хендлеры, валидация, writeError                           [автор]
│   ├── worker/             поллер, семафор, run, дерево контекстов, shutdown        [автор]
│   ├── handlers/           реестр, webhook, sleep                                    [автор]
│   └── metrics/            метрики и Collector глубины очереди                      [автор]
├── migrations/             SQL-миграции goose + embed.go                            [Claude]
├── scripts/chaos.sh        chaos-демо                                                [Claude]
├── Dockerfile, docker-compose.yml, Makefile, .golangci.yml,
│   .github/workflows/ci.yml, README.md                                              [Claude]
└── docs/superpowers/specs/ эта спека
```

**Зависимости между пакетами:** `api → job, metrics`, `worker → job, metrics`, `store → job`, `handlers → job`, `metrics → job`. Пакеты `api` и `worker` не знают ни друг о друге, ни о `store` и `handlers`, всё связывает `main`. Общаются `api` и `worker` только через таблицу `jobs`.

**Интерфейсы объявляет потребитель:**
- `worker` объявляет свой интерфейс `Store` (`Claim`, `Complete`, `Fail`) и `Handlers` (`Get(type) (Handler, bool)`);
- `api` объявляет свой интерфейс `Store` (`Create`, `Get`, `List`, `Retry`) и `Validator` (`Validate(type string, payload json.RawMessage) error`);
- пакет `store` экспортирует конкретный тип `*store.Postgres` и ни о каких интерфейсах не знает.

**Интерфейс обработчика** (`handlers`):
```go
type Handler interface {
    Handle(ctx context.Context, payload json.RawMessage) error
}
// Необязательный интерфейс: если обработчик его реализует,
// реестр проверяет payload ещё при создании задачи (проверка через type assertion).
type PayloadValidator interface {
    ValidatePayload(payload json.RawMessage) error
}
```
Реестр `handlers.Registry` реализует и `Validator` для `api` (неизвестный тип или невалидный payload дают ошибку), и `Handlers` для `worker`. Чтобы добавить новый тип задачи, достаточно нового файла в `handlers` и регистрации в `main`.

## 5. HTTP API

Формат JSON-представления задачи:
```json
{
  "id": 42, "type": "webhook", "payload": {"url": "https://…", "body": {}},
  "status": "pending", "attempts": 0, "max_attempts": 5,
  "run_at": "2026-10-25T12:00:00Z", "last_error": null,
  "created_at": "…", "updated_at": "…", "finished_at": null
}
```

| Метод и путь | Тело или параметры | Ответы |
|---|---|---|
| `POST /jobs` | `type` (обяз.), `payload` (обяз.), `run_at` (RFC 3339, по умолчанию «сейчас»), `max_attempts` (1–25, по умолчанию 5), `idempotency_key` (необяз.) | `201` создана; `200` задача с таким `idempotency_key` уже есть, возвращаем её; `400` ошибка валидации; `413` payload больше 64 КБ |
| `GET /jobs/{id}` | — | `200`, `404` |
| `GET /jobs` | `status` (необяз.), `limit` (1–200, по умолчанию 50), `after_id` (необяз.) | `200 {"jobs": [...], "next_after_id": 17}`. Keyset-пагинация: `ORDER BY id DESC`, `WHERE id < after_id`; `next_after_id` равен `null`, если данных больше нет |
| `POST /jobs/{id}/retry` | — | `200` задача снова в `pending`; `404`; `409`, если статус не `dead` |
| `GET /healthz` | — | всегда `200` (liveness) |
| `GET /readyz` | — | `200`, если пинг БД успешен и shutdown не начат; иначе `503` |
| `GET /metrics` | — | Prometheus |

Ошибки всегда возвращаются в виде `{"error": "<сообщение>"}`.

**Payload по типам:**
- `webhook`: `{"url": "<http(s) URL>", "body": <любой JSON>}`. Отправляется `POST url` с телом `body` и заголовками `Content-Type: application/json`, `X-Taskq-Job-Id`, `X-Taskq-Attempt`.
- `sleep`: `{"seconds": <0–300>, "fail_probability": <0–1>, "panic_probability": <0–1>}`. Спит, учитывая `ctx`, затем с заданной вероятностью падает или паникует.

## 6. Модель данных

```sql
CREATE TABLE jobs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type            TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'succeeded', 'dead')),
    attempts        INT         NOT NULL DEFAULT 0,
    max_attempts    INT         NOT NULL DEFAULT 5 CHECK (max_attempts BETWEEN 1 AND 25),
    run_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by       TEXT,
    locked_until    TIMESTAMPTZ,
    last_error      TEXT,
    idempotency_key TEXT UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ
);
CREATE INDEX jobs_pending_run_at_idx    ON jobs (run_at)       WHERE status = 'pending';
CREATE INDEX jobs_running_lease_idx     ON jobs (locked_until) WHERE status = 'running';
```

**Единые часы — часы БД.** Все времена (`run_at`, `locked_until`, `finished_at`, следующий запуск при повторе) вычисляются через `now()` в Postgres. Go передаёт только длительности. Так часы разных воркеров не расходятся.

## 7. Жизненный цикл задачи

```
            ┌──────────── retry (вручную, только из dead) ─────────────┐
            ▼                                                          │
 POST → pending ──claim──► running ──успех──► succeeded                │
            ▲                 │                                        │
            │                 ├─ ошибка, попытки остались → pending (run_at = now()+backoff)
            │                 ├─ ошибка, попытки кончились или Permanent → dead ──┘
            └── аренда истекла (воркер упал или завис) ──┘
```

### 7.1 Claim
Один атомарный запрос забирает до `n` задач:
```sql
UPDATE jobs
SET status = 'running', locked_by = $1, locked_until = now() + $2,
    attempts = attempts + 1, updated_at = now()
WHERE id IN (
    SELECT id FROM jobs
    WHERE (status = 'pending' AND run_at <= now())
       OR (status = 'running' AND locked_until < now())
    ORDER BY run_at
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING <все колонки>;
```
- `attempts` увеличивается при claim, поэтому задача, которая роняет воркер, тоже расходует попытки.
- Если после claim `attempts > max_attempts` (такое бывает только у подобранной «осиротевшей» задачи), воркер её **не выполняет** и сразу переводит в `dead` с ошибкой `max attempts exceeded (lease expired)`.

### 7.2 Аренда
`lease = JOB_TIMEOUT + LEASE_MARGIN` (по умолчанию 30s + 10s). Аренда не продлевается. Задача, которая выполняется дольше аренды, считается «зависшей» и может быть подобрана другим воркером.

### 7.3 Fencing
`Complete` и `Fail` обновляют задачу только при условии `WHERE id = $1 AND locked_by = $2 AND status = 'running'`. Если обновилось 0 строк, задачей владеет кто-то другой. Тогда метод возвращает `job.ErrLeaseLost`, воркер пишет warning и больше ничего не делает.

### 7.4 Решение после ошибки: `job.NextStep`
Чистая функция в пакете `job`: на входе задача и ошибка, на выходе `Retry(delay)` или `Dead`.
- `errors.As(err, &PermanentError)` → `Dead`;
- `attempts >= max_attempts` → `Dead`;
- иначе → `Retry(Backoff(attempts))`.

### 7.5 Backoff
`d = min(1s · 2^(attempt−1), 10m)`. Результат — `d/2 + rand[0, d/2)` (equal jitter). Источник случайности передаётся параметром, чтобы тесты были детерминированными.

### 7.6 Fail
- При `Retry(delay)`: `status='pending'`, `run_at = now() + delay`, `last_error`, `locked_by = NULL`, `locked_until = NULL`.
- При `Dead`: `status='dead'`, `finished_at = now()`, `last_error`, снимаем блокировку.

### 7.7 Идемпотентность создания
`INSERT … ON CONFLICT (idempotency_key) DO NOTHING RETURNING …`. Если строка не вернулась, отдельным `SELECT` получаем существующую задачу по ключу. API отвечает `200` вместо `201`. Параметры существующей задачи с параметрами запроса не сравниваем.

### 7.8 Ручной retry
Только из `dead`: `status='pending'`, `attempts=0`, `run_at=now()`, `last_error` сохраняем, `finished_at=NULL`. Из других статусов возвращаем `job.ErrConflict` (HTTP 409).

### 7.9 Семантика доставки
At-least-once. Дубли возможны: воркер упал после выполнения, но до `Complete`, или аренда истекла у зависшего воркера. Получатель webhook отсеивает дубли по заголовку `X-Taskq-Job-Id`.

## 8. Конкурентность воркера

```
signal.NotifyContext(SIGINT, SIGTERM) → rootCtx
        │
   Poller (1 горутина)
        │  1) блокируется, пока не освободится хотя бы один слот (семафор: chan struct{} на N мест)
        │  2) без блокировки занимает остальные свободные слоты → n
        │  3) Claim(n); неиспользованные слоты освобождает
        │  4) на каждую задачу: wg.Go(run), слот освобождается в defer
        │  5) задач 0 → ждёт POLL_INTERVAL ± 20% jitter (select с rootCtx.Done)
        ▼
   до N горутин run(job)
```

### 8.1 Правила
- Задач забираем ровно столько, сколько свободных слотов. Внутреннего буфера задач нет.
- Ошибка `Claim` не завершает процесс: пишем в лог, увеличиваем `taskq_claim_errors_total`, освобождаем слоты, ждём `POLL_INTERVAL` и пробуем снова.
- Обработчик берётся из реестра по `type`. Если тип неизвестен, задача сразу уходит в `dead` (Permanent).

### 8.2 Дерево контекстов
- `rootCtx` отменяется по сигналу, от него зависит **только поллер**.
- `workCtx` — отдельный контекст от `context.Background()`. Он отменяется только по истечении `SHUTDOWN_TIMEOUT`.
- Задача выполняется в `jobCtx = context.WithTimeout(workCtx, JOB_TIMEOUT)`.
- `Complete` и `Fail` выполняются в `context.WithTimeout(context.WithoutCancel(jobCtx), 5s)`. Даже если задачу отменили, её результат всё равно запишется.

### 8.3 Graceful shutdown воркера
1. SIGTERM отменяет `rootCtx`, поллер выходит из цикла, новых Claim нет.
2. Ждём `wg` не дольше `SHUTDOWN_TIMEOUT` (25s; в compose `stop_grace_period: 30s`).
3. Если время вышло, вызываем `cancelWork()`. Обработчики получают отмену, задачи идут в `Fail` по общему пути (попытка засчитывается, это осознанное упрощение). После этого ждём `wg` до конца.
4. Закрываем `pgxpool` и HTTP-сервер метрик, выходим с кодом 0.

### 8.4 Graceful shutdown API
По сигналу `/readyz` сразу начинает отдавать `503`. Затем `http.Server.Shutdown` с таймаутом `SHUTDOWN_TIMEOUT`, закрытие `pgxpool` и выход с кодом 0.

### 8.5 Защита от падений
- В `run` стоит `defer recover()`. Паника превращается в обычную ошибку `panic: <значение>`, полный стек пишется в лог, дальше обычный путь `NextStep`.
- `webhook`: один общий `http.Client` (таймаут `JOB_TIMEOUT`), запрос через `http.NewRequestWithContext(jobCtx, …)`, тело ответа всегда вычитываем (`io.Copy(io.Discard, …)`) и закрываем. Коды ответа: 2xx — успех; 429 и 5xx — обычная ошибка (повтор); остальные 4xx — `PermanentError`. Сетевые ошибки и таймауты — повтор.

## 9. Ошибки

- Пакет `job`: `ErrNotFound`, `ErrConflict`, `ErrLeaseLost`, `ValidationError` (с полем и сообщением), `PermanentError` (обёртка с `Unwrap`) и конструктор `job.Permanent(err)`.
- `store` превращает `pgx.ErrNoRows` в `job.ErrNotFound`. Ошибки драйвера наружу не выходят «как есть», их всегда оборачиваем: `fmt.Errorf("store: claim: %w", err)`.
- В `api` ошибки переводятся в HTTP-коды в одном месте, функции `writeError`: `ValidationError` → 400, `ErrNotFound` → 404, `ErrConflict` → 409, `*http.MaxBytesError` → 413, всё остальное → 500. Текст 500-й ошибки клиенту не показываем, только пишем в лог.

## 10. Наблюдаемость

**Логи:** `slog.JSONHandler`, уровень задаётся через `LOG_LEVEL`. Поля: `request_id` (API), `job_id`, `type`, `attempt`, `worker_id`, `duration_ms`, `error`.

**Метрики:**

| Метрика | Тип | Процесс |
|---|---|---|
| `taskq_jobs_created_total{type}` | counter | api |
| `taskq_queue_depth{status}` | gauge, собственный `prometheus.Collector`: на каждый сбор делает `CountByStatus` с таймаутом 2s | api |
| `taskq_jobs_processed_total{type,result}` (`result`: `succeeded`, `retry`, `dead`) | counter | worker |
| `taskq_job_duration_seconds{type}` | histogram | worker |
| `taskq_jobs_in_flight` | gauge | worker |
| `taskq_claim_errors_total` | counter | worker |
| `taskq_lease_lost_total` | counter | worker |

API отдаёт `/metrics` на `HTTP_ADDR`, воркер — на `METRICS_ADDR`. Prometheus в compose опрашивает api и все реплики воркера (через DNS-обнаружение сервиса `worker`).

## 11. Конфигурация (env)

| Переменная | По умолчанию | Где |
|---|---|---|
| `DATABASE_URL` | — (обязательна) | все |
| `LOG_LEVEL` | `info` | все |
| `HTTP_ADDR` | `:8080` | api |
| `METRICS_ADDR` | `:9090` | worker |
| `WORKER_ID` | `<hostname>-<pid>` | worker |
| `WORKER_CONCURRENCY` | `10` | worker |
| `POLL_INTERVAL` | `1s` | worker |
| `JOB_TIMEOUT` | `30s` | worker |
| `LEASE_MARGIN` | `10s` | worker |
| `SHUTDOWN_TIMEOUT` | `25s` | api, worker |

Если конфиг невалиден, процесс завершается при старте с понятной ошибкой.

## 12. Тестирование

| Уровень | Что проверяем | Инструменты |
|---|---|---|
| Unit: `job` | `Backoff` (границы, потолок, jitter с фиксированным rand), `NextStep` | табличные тесты |
| Unit: `api` | все эндпоинты и коды ответа, валидация, 413, идемпотентный `200` | `httptest`, fake store и fake validator |
| Unit: `handlers` | webhook: 2xx, 4xx → Permanent, 429 и 5xx → повтор, таймаут, заголовки; sleep учитывает отмену `ctx`; валидация payload | `httptest.Server` |
| Unit: `worker` | не больше N задач параллельно; Claim просит ровно столько задач, сколько свободных слотов; пустая очередь → пауза; ошибка Claim не роняет процесс; shutdown ждёт текущие задачи; по таймауту shutdown срабатывает отмена; паника перехватывается; итоговая запись проходит после отмены `jobCtx`; `ErrLeaseLost` только пишется в лог; `attempts > max_attempts` → `dead` без запуска | fake store, `testing/synctest` |
| Интеграционные: `store` | CRUD и идемпотентность; 1000 задач и 10 горутин-claimer'ов → каждую задачу забрали ровно один раз; истёкшую аренду подбирает другой воркер; fencing (0 строк → `ErrLeaseLost`); `Fail` → `pending` с правильным `run_at` или `dead`; `Retry` из `dead` и `409` из других статусов; keyset-пагинация | `testcontainers-go`, build tag `integration` |
| Chaos-демо | `compose up --scale worker=3`, 500 задач `sleep` (часть падает, часть паникует), `docker kill` одного воркера посреди работы → все задачи в `succeeded` или `dead`, итог печатается в README | `scripts/chaos.sh` |

**Команды:** `make test` (`go test -race ./...`), `make test-integration` (`-tags=integration`), `make lint`, `make chaos`.

**CI (GitHub Actions):** три задачи — `lint`, `test`, `integration` (на `ubuntu-latest` есть Docker). Все обязательны для мержа в `main`.

## 13. План работ и процесс

### Процесс одного шага
1. Claude выдаёт **карточку задачи**: цель, критерии приёмки, подсказки, что почитать.
2. Автор работает в ветке `wN/<тема>` и открывает PR в `main`.
3. Claude делает ревью как строгий тимлид, автор исправляет замечания, мержим.
4. **Definition of Done для PR:** линтер чистый; `go test -race` зелёный; новое поведение покрыто тестами; README обновлён, если поменялось поведение, видимое снаружи.
5. **В конце недели:** 5 вопросов с собеседования по теме недели, автор отвечает, Claude оценивает и указывает пробелы.

### Недели

| Неделя | Кто | Объём | Что прокачивает |
|---|---|---|---|
| 0 | Claude | `go.mod` и линтер через `go tool`; `cmd/taskq` с подкомандами; `config`; миграция `jobs`; Dockerfile и compose (postgres, migrate, api, worker, prometheus); Makefile; CI; роутер и middleware; README; пустые пакеты с описанием их назначения в `doc.go` | — |
| 1 | автор | `job`: типы, ошибки, `Backoff`, `NextStep` + unit-тесты. `store`: `Create`, `Get`, `List`, идемпотентность + интеграционные тесты | структуры, методы, ошибки, pgx, табличные тесты, testcontainers |
| 2 | автор | `api`: хендлеры, `writeError`, валидация, unit-тесты на fake. `store`: `Claim`, `Complete`, `Fail`, `Retry`, `CountByStatus` + тест «1000 задач на 10 claimer'ов» | `net/http`, JSON, интерфейсы на стороне потребителя, `SKIP LOCKED` |
| 3 | автор | `handlers`: реестр, `sleep`, `webhook`. `worker`: поллер, семафор, `run`, `recover`, контексты, shutdown + тесты на `synctest` | горутины, каналы, `select`, `context`, `WaitGroup`, `-race` (самая тяжёлая неделя) |
| 4 | автор + Claude | Автор: `metrics` и `Collector`, подключение метрик. Claude: `scripts/chaos.sh`. Вместе: раздел README «Почему так» (SKIP LOCKED, at-least-once, fencing, единые часы БД, дерево контекстов) | Prometheus, умение защитить свои решения |

## 14. Риски и осознанные упрощения

- **Повтор после принудительного shutdown расходует попытку.** Это проще, чем отдельный `Release`. Ограничение документируем в README.
- **Аренда не продлевается.** Если задача выполняется дольше `JOB_TIMEOUT + LEASE_MARGIN`, её может выполнить второй воркер. Защита: таймаут задачи меньше аренды, а fencing не даёт первому воркеру перезаписать результат. Полное решение (heartbeat) — этап 2.
- **Опрос базы вместо push-уведомлений.** При `POLL_INTERVAL=1s` и небольшом числе воркеров нагрузка на БД пренебрежимо мала. `LISTEN/NOTIFY` — этап 2.
- **Неделя 3 может не уложиться в 10 часов.** Допускаем, что она растянется на полторы недели. Сокращать объём в ней нельзя, ради неё проект и затевался.
