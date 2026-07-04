# Team Task Tracker API

REST API сервис для управления задачами в командах с PostgreSQL, Redis, JWT, аудитом изменений, сложными SQL-запросами, Prometheus-метриками и Vue-фронтендом.

## Быстрый старт

```bash
docker compose up --build
```

- API: http://localhost:8080
- Метрики: http://localhost:8080/metrics
- Frontend: http://localhost:5173

## Основные эндпоинты

- `POST /api/v1/register`
- `POST /api/v1/register/request-code`
- `POST /api/v1/register/verify`
- `POST /api/v1/login`
- `POST /api/v1/teams`
- `GET /api/v1/teams`
- `GET /api/v1/teams/{id}/workers`
- `POST /api/v1/teams/{id}/workers`
- `PUT /api/v1/teams/{id}/workers/{worker_id}`
- `DELETE /api/v1/teams/{id}/workers/{worker_id}`
- `POST /api/v1/teams/{id}/auto-assign`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks?team_id=1&status=todo&assignee_id=5&limit=20&offset=0`
- `PUT /api/v1/tasks/{id}`
- `DELETE /api/v1/tasks/{id}`
- `GET /api/v1/tasks/{id}/history`
- `GET /api/v1/tasks/{id}/comments`
- `POST /api/v1/tasks/{id}/comments`
- `GET /api/v1/reports/team-summary`
- `GET /api/v1/reports/top-creators`
- `GET /api/v1/reports/invalid-assignees`

## Качество

```bash
go test ./...
```

Интеграционные тесты используют testcontainers и требуют Docker.
