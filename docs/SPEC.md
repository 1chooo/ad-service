# ad-service Specification

Design and implement a simplified **Advertisement Delivery Service** using **Golang**. The service exposes two main APIs: one for creating advertisements with targeting conditions, and one for listing matching advertisements based on a user's profile.

---

## Repository Structure

Minimal, idiomatic Go layout:

```text
ad-service/
├── cmd/
│   └── server/
│       └── main.go          # Application entry point
├── internal/
│   ├── delivery/
│   │   └── http/            # API route handlers (Admin & Public)
│   ├── model/
│   │   └── ad.go            # Data structures & validation (Ad, Conditions)
│   ├── repository/
│   │   └── ad_repo.go       # Storage layer logic (DB/Cache)
│   └── service/
│       └── ad_service.go    # Core business logic & filtering algorithms
├── docs/
│   └── SPEC.md              # This document
├── go.mod
├── go.sum
└── README.md                # System design & thought process
```

---

## Service API Specifications

### 1. Admin API — Create Advertisement

| Property | Value |
|----------|-------|
| **Method** | `POST` |
| **Path** | `/api/v1/ad` |
| **Purpose** | Create advertisement resources. Only **Create** is required (no list, update, or delete). |

#### Request Body

Each advertisement must include the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | Yes | Title of the advertisement. |
| `startAt` | `string` (ISO 8601) | Yes | Start of the active display window. |
| `endAt` | `string` (ISO 8601) | Yes | End of the active display window. Must be after `startAt`. |
| `conditions` | `object` | No | Targeting criteria (see below). |

#### Conditions (Targeting Criteria)

Every condition is **optional**. If a condition is omitted or empty, there is **no restriction** for that attribute.

Each condition may accept **multiple values** (e.g., an ad can target both `TW` and `JP`).

| Category | Type | Constraints |
|----------|------|-------------|
| **Age** | Range | `ageStart` and `ageEnd`, each between `1` and `100` (inclusive). |
| **Gender** | Enum array | `M`, `F` |
| **Country** | Enum array | ISO 3166-1 alpha-2 codes (e.g., `TW`, `JP`). |
| **Platform** | Enum array | `android`, `ios`, `web` |

#### Example Request

Create an ad targeting users aged 20–30, in Taiwan or Japan, on Android or iOS, with no gender restriction:

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

---

### 2. Public Delivery API — List Matching Ads

| Property | Value |
|----------|-------|
| **Method** | `GET` |
| **Path** | `/api/v1/ad` |
| **Purpose** | Return active, matching advertisements for the given user profile. |

#### Active Definition

An ad is **active** when:

```
StartAt < NOW < EndAt
```

#### Query Parameters

| Parameter | Required | Default | Constraints |
|-----------|----------|---------|-------------|
| `offset` | No | `0` | Non-negative integer. |
| `limit` | Yes* | `5` | Integer between `1` and `100`. |
| `age` | No | — | Integer between `1` and `100`. |
| `gender` | No | — | `M` or `F`. |
| `country` | No | — | ISO 3166-1 alpha-2 code. |
| `platform` | No | — | `android`, `ios`, or `web`. |

\* `limit` is required in the sense that it must be validated when provided; when omitted, default to `5`.

#### Matching Logic

An ad matches a request when **all** of the following hold:

1. The ad is **active** at request time.
2. For each provided user attribute, the ad's corresponding condition is either **unset** (no restriction) or **includes** the user's value:
   - **Age:** `ageStart ≤ userAge ≤ ageEnd` (if either bound is set).
   - **Gender:** user's gender is in the ad's `gender` list (if set).
   - **Country:** user's country is in the ad's `country` list (if set).
   - **Platform:** user's platform is in the ad's `platform` list (if set).

#### Sorting

Results are sorted by `endAt` in **ascending** order (earliest ending ad first).

#### Pagination

Use `offset` and `limit` query parameters. Return at most `limit` items after skipping `offset` records from the sorted result set.

#### Example Request

```bash
curl -X GET \
  "http://localhost:8080/api/v1/ad?offset=10&limit=3&age=24&gender=F&country=TW&platform=ios"
```

#### Example Response

Returns up to `limit` matching active ads:

```json
{
  "items": [
    {
      "title": "AD 1",
      "endAt": "2026-06-22T01:00:00.000Z"
    },
    {
      "title": "AD 31",
      "endAt": "2026-06-30T12:00:00.000Z"
    },
    {
      "title": "AD 10",
      "endAt": "2026-06-30T16:00:00.000Z"
    }
  ]
}
```

Public list responses expose only `title` and `endAt` per item.

---

## System Constraints & Requirements

### Performance

- The Public API must handle **10,000+ requests per second (RPS)**.

### Data Scale

| Metric | Limit |
|--------|-------|
| Concurrent active ads (`StartAt < NOW < EndAt`) | < 1,000 |
| New ads created per day | ≤ 3,000 |

### Validation & Errors

- Validate all request parameters and body fields for both APIs.
- Return appropriate HTTP status codes and error messages for invalid input.

### Testing

- Write unit tests covering business logic, validation, filtering, sorting, and pagination.

### Authentication

- **Not required.** No auth or authorization mechanisms need to be implemented.

### Tech Stack

- External libraries are allowed.
- Storage is your choice, for example:
  - **Databases:** MySQL, PostgreSQL, MongoDB
  - **Cache:** Redis, Memcached

### Documentation

- Maintain a `README.md` describing architecture, design choices, and thought process.

---

## Grading Criteria

| Area | Expectation |
|------|-------------|
| **Correctness** | Meets all basic requirements and behaves as specified. |
| **Performance** | Architecture and implementation meet or exceed 10k+ RPS on the Public API. |
| **Readability** | Clean code, helpful comments where needed, clear documentation. |
| **Testing** | Thorough unit test coverage of core behavior. |

---

## Validation Reference

### Admin API — Create

| Field | Rules |
|-------|-------|
| `title` | Non-empty string. |
| `startAt` | Valid ISO 8601 timestamp. |
| `endAt` | Valid ISO 8601 timestamp; must be after `startAt`. |
| `conditions.ageStart` | Optional; if set, integer 1–100. |
| `conditions.ageEnd` | Optional; if set, integer 1–100; must be ≥ `ageStart` when both set. |
| `conditions.gender` | Optional; each value must be `M` or `F`. |
| `conditions.country` | Optional; each value must be a valid ISO 3166-1 alpha-2 code. |
| `conditions.platform` | Optional; each value must be `android`, `ios`, or `web`. |

### Public API — List

| Parameter | Rules |
|-----------|-------|
| `offset` | Optional; non-negative integer; default `0`. |
| `limit` | Optional; integer 1–100; default `5`. |
| `age` | Optional; integer 1–100. |
| `gender` | Optional; `M` or `F`. |
| `country` | Optional; valid ISO 3166-1 alpha-2 code. |
| `platform` | Optional; `android`, `ios`, or `web`. |

---

## Error Response Format (Recommended)

Use a consistent JSON error shape across both APIs:

```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "limit must be between 1 and 100"
  }
}
```

Suggested HTTP status codes:

| Status | Usage |
|--------|-------|
| `400 Bad Request` | Validation failures, malformed JSON. |
| `405 Method Not Allowed` | Wrong HTTP method on a route. |
| `500 Internal Server Error` | Unexpected server failures. |
