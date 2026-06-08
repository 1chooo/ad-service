# ad-service

A simplified **Advertisement Delivery Service** in Go. It exposes an admin API to create targeted ads and a public API to list matching active ads for a user profile.

## Architecture

```text
Client
  |  POST /api/v1/ad  (create)
  |  GET  /api/v1/ad  (list matching)
  v
HTTP handlers (chi)
  v
AdService (validation, filtering, sorting, pagination)
  v
AdRepository
  |-- PostgreSQL  (durable writes, active-ad reload)
  '-- In-memory cache  (read path for high throughput)
```

### Design choices

- **PostgreSQL via Docker** stores all ads durably (~3,000 creates/day).
- **In-memory active-ad cache** serves the public list API without hitting the DB on every request. With fewer than 1,000 concurrent active ads, in-memory filter + sort + paginate is sufficient for **10k+ RPS**.
- A **1-second background refresher** reloads active ads from PostgreSQL so newly started ads appear and expired ads are evicted without requiring a restart.

## Project layout

```text
cmd/server/main.go              Application entry point
internal/delivery/http/         HTTP handlers and routing
internal/model/ad.go            Data structures, validation, matching
internal/repository/ad_repo.go  PostgreSQL + active-ad cache
internal/service/ad_service.go  Business logic
docs/SPEC.md                    Full API specification
```

## Prerequisites

- Go 1.25+
- Docker and Docker Compose

## Run locally

Start PostgreSQL:

```bash
docker compose up -d postgres
```

Run the server:

```bash
go run ./cmd/server
```

The server listens on `:8080` by default. Configure with environment variables:

| Variable | Default |
|----------|---------|
| `PORT` | `8080` |
| `DATABASE_URL` | `postgres://ad:ad@localhost:5432/ad_service?sslmode=disable` |

### Run everything in Docker

```bash
docker compose up --build
```

## API examples

### Create an ad

```bash
curl -X POST -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/ad" \
  --data '{
    "title": "AD 55",
    "startAt": "2026-06-10T03:00:00.000Z",
    "endAt": "2026-06-30T16:00:00.000Z",
    "conditions": {
      "ageStart": 20,
      "ageEnd": 30,
      "country": ["TW", "JP"],
      "platform": ["android", "ios"]
    }
  }'
```

### List matching ads

```bash
curl -X GET \
  "http://localhost:8080/api/v1/ad?offset=0&limit=3&age=24&gender=F&country=TW&platform=ios"
```

Example response:

```json
{
  "items": [
    { "title": "AD 1", "endAt": "2026-06-22T01:00:00.000Z" },
    { "title": "AD 31", "endAt": "2026-06-30T12:00:00.000Z" }
  ]
}
```

### Error format

```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "limit must be between 1 and 100"
  }
}
```

## Testing

```bash
go test ./...
```

Unit tests cover validation, matching logic, sorting, pagination, and service behavior.

## Trade-offs and extensions

| Area | Current approach | Possible extension |
|------|------------------|--------------------|
| Reads | In-process cache | Redis shared cache across replicas |
| Writes | Single PostgreSQL | Read replicas, connection pooling at scale |
| Matching | In-memory scan | Pre-indexed segments by country/platform |
| Auth | None (per spec) | API keys or mTLS for admin routes |

See [docs/SPEC.md](docs/SPEC.md) for the full specification.
