# taskq

Очередь фоновых задач на Go и Postgres: HTTP API для постановки задач, пул воркеров с повторами,
арендой (lease) и graceful shutdown, метрики Prometheus.

> Учебный проект: цель — пройти путь от Junior к Middle в Go на задаче, где конкурентность —
> в центре, а не сбоку. Дизайн и решения: [спека](docs/superpowers/specs/2026-10-25-taskq-design.md),
> план работ: [план](docs/superpowers/plans/2026-10-25-taskq-mvp.md).

**Статус:** неделя 0 — каркас (API отвечает на health-эндпоинты, воркер пока заглушка).

## Быстрый старт

Нужны Docker и Go 1.27+.

```bash
make up                                  # postgres, миграции, api, worker, prometheus
curl -s localhost:8080/healthz           # {"status":"ok"}
curl -s localhost:8080/readyz            # {"status":"ready"}
docker compose up -d --scale worker=3    # несколько воркеров
open http://localhost:9090/targets       # Prometheus видит api и все воркеры
make down
```

## Разработка

```bash
make test              # unit-тесты с -race
make test-integration  # + интеграционные тесты на настоящем Postgres (testcontainers, нужен Docker)
make lint              # golangci-lint v2, версия закреплена в tools/go.mod
```

## Устройство

Один бинарник `taskq` с подкомандами `api`, `worker`, `migrate up`. API и воркеры общаются только
через таблицу `jobs` в Postgres. Подробности — в [спеке](docs/superpowers/specs/2026-10-25-taskq-design.md).
