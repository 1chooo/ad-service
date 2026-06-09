# ad-service Specification

Design and implement a simplified **Advertisement Delivery Service** using **Golang**. The service exposes two main APIs: one for creating advertisements with targeting conditions, and one for listing matching advertisements based on a user's profile.

---

## Repository Structure

Minimal, idiomatic Go layout:

```text
ad-service/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── delivery/
│   │   └── http/                # API route handlers (Admin & Public)
│   ├── model/
│   │   └── ad.go                # Data structures & validation (Ad, Conditions)
│   ├── repository/
│   │   └── ad_repo.go           # Storage layer logic (DB/Cache)
│   └── service/
│       └── ad_service.go        # Core business logic & filtering algorithms
├── docs/
│   └── SPEC.md                  # This document
├── go.mod
├── go.sum
└── README.md                    # System design & thought process
```

---

## Service API Specifications

### 1. Admin API — Create Advertisement

| Property | Value |
|----------|-------|
| **Method** | `POST` |
| **Path** | `/api/v1/ad` |
| **Purpose** | Create advertisement resources. |

#### Request Body

Each advertisement must include the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | `string` | Yes | Title of the advertisement. |
| `description` | `string` | No | Body/description text of the ad. |
| `imageUrl` | `string` | No | URL to the ad creative image. |
| `landingPageUrl` | `string` | No | Destination URL when the ad is clicked. |
| `bid` | `number` | No | CPM bid price. Higher bid = higher priority in delivery ranking. |
| `dailyBudget` | `integer` | No | Maximum number of daily impressions for this ad. |
| `status` | `string` | No | Lifecycle state: `active`, `paused`, or `archived`. Defaults to `active`. |
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
| **Exclude Gender** | Enum array | `M`, `F` — users matching these values are excluded. |
| **Exclude Country** | Enum array | ISO 3166-1 alpha-2 codes — users in these countries are excluded. |
| **Exclude Platform** | Enum array | `android`, `ios`, `web` — users on these platforms are excluded. |
| **Daypart Start** | Time (HH:MM) | Start of the daily time window (e.g., `09:00`). Must be paired with `daypartEnd`. |
| **Daypart End** | Time (HH:MM) | End of the daily time window (e.g., `17:00`). Must be paired with `daypartStart`. |

Exclusion rules are evaluated **before** inclusion rules. If an exclusion condition matches, the ad is excluded regardless of inclusion conditions.

Dayparting supports overnight windows (e.g., `22:00`–`06:00`) where `daypartStart > daypartEnd`.

#### Example Request

Create an ad targeting users aged 20–30, in Taiwan or Japan, on Android or iOS, with no gender restriction, a bid of $2.50 CPM, and a daily budget of 10,000 impressions:

```bash
curl -X POST -H "Content-Type: application/json" \
  "http://localhost:8080/api/v1/ad" \
  --data '{
    "title": "AD 55",
    "description": "Check out our latest product!",
    "imageUrl": "https://cdn.example.com/ad55.jpg",
    "landingPageUrl": "https://example.com/landing",
    "bid": 2.50,
    "dailyBudget": 10000,
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

#### Idempotency

The Admin API supports optional idempotency via the `Idempotency-Key` header. If a request with the same key is received within 5 minutes, the previously created ad is returned instead of creating a duplicate.

```bash
curl -X POST -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-unique-key-123" \
  "http://localhost:8080/api/v1/ad" \
  --data '{
    "title": "AD 55",
    "startAt": "2026-06-10T03:00:00.000Z",
    "endAt": "2026-06-30T16:00:00.000Z"
  }'
```

---

### 2. Admin API — Bulk Create Advertisements

| Property | Value |
|----------|-------|
| **Method** | `POST` |
| **Path** | `/api/v1/ads` |
| **Purpose** | Create multiple advertisements in a single request. |

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ads` | `array` | Yes | Array of ad creation requests (same schema as single create). |

#### Response Body

```json
{
  "ads": [
    { "id": 1, "title": "Ad 1", ... },
    { "id": 2, "title": "Ad 2", ... }
  ],
  "failures": [
    { "index": 2, "error": "title must be a non-empty string" }
  ]
}
```

---

### 3. Public Delivery API — List Matching Ads

| Property | Value |
|----------|-------|
| **Method** | `GET` |
| **Path** | `/api/v1/ad` |
| **Purpose** | Return active, matching advertisements for the given user profile. |

#### Active Definition

An ad is **active** when:

```
Status = "active" AND StartAt < NOW < EndAt
```

Ads with status `paused` or `archived` are excluded from delivery.

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

1. The ad is **active** at request time (status = `active` and within time window).
2. For each provided user attribute, the ad does **not** exclude the user, and the ad's corresponding condition is either **unset** (no restriction) or **includes** the user's value:
   - **Age:** `ageStart ≤ userAge ≤ ageEnd` (if either bound is set).
   - **Gender:** user's gender is not in `excludeGender`; if `gender` is set, user's gender must be in the list.
   - **Country:** user's country is not in `excludeCountry`; if `country` is set, user's country must be in the list.
   - **Platform:** user's platform is not in `excludePlatform`; if `platform` is set, user's platform must be in the list.
3. If dayparting is configured, the current time falls within the specified daily window.
4. The ad has not exceeded its `dailyBudget` (if set).

#### Sorting

Results are sorted by:
1. **Bid** descending (highest bid first, where `bid > 0`).
2. **EndAt** ascending (earliest ending ad first) as tiebreaker.

#### Pagination

Use `offset` and `limit` query parameters. Return at most `limit` items after skipping `offset` records from the sorted result set.

#### Budget Tracking

When `dailyBudget` is set on an ad, the service tracks impressions served per ad per day in-memory. Once the impression count reaches the budget, the ad stops being delivered until the next day.

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
      "description": "Check out our latest product!",
      "imageUrl": "https://cdn.example.com/ad1.jpg",
      "landingPageUrl": "https://example.com/ad1",
      "endAt": "2026-06-22T01:00:00.000Z"
    },
    {
      "title": "AD 31",
      "endAt": "2026-06-30T12:00:00.000Z"
    }
  ]
}
```

Public list responses expose `title`, `endAt`, and optionally `description`, `imageUrl`, `landingPageUrl` per item.

---

### 4. Admin API — Rate Limiting

Admin endpoints (`POST /api/v1/ad` and `POST /api/v1/ads`) are rate-limited per IP address. The default limit is **100 requests per minute** per client IP. When exceeded, the API returns HTTP `429 Too Many Requests`:

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "too many requests, try again later"
  }
}
```

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

- Write unit tests covering business logic, validation, filtering, sorting, pagination, exclusion targeting, dayparting, bid-based sorting, budget tracking, bulk creation, and lifecycle status.

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
| `description` | Optional string. |
| `imageUrl` | Optional string. |
| `landingPageUrl` | Optional string. |
| `bid` | Optional; non-negative float. Defaults to `0`. |
| `dailyBudget` | Optional; non-negative integer. |
| `status` | Optional; must be `active`, `paused`, or `archived`. Defaults to `active`. |
| `startAt` | Valid ISO 8601 timestamp. |
| `endAt` | Valid ISO 8601 timestamp; must be after `startAt`. |
| `conditions.ageStart` | Optional; if set, integer 1–100. |
| `conditions.ageEnd` | Optional; if set, integer 1–100; must be ≥ `ageStart` when both set. |
| `conditions.gender` | Optional; each value must be `M` or `F`. |
| `conditions.country` | Optional; each value must be a valid ISO 3166-1 alpha-2 code. |
| `conditions.platform` | Optional; each value must be `android`, `ios`, or `web`. |
| `conditions.excludeGender` | Optional; each value must be `M` or `F`. |
| `conditions.excludeCountry` | Optional; each value must be a valid ISO 3166-1 alpha-2 code. |
| `conditions.excludePlatform` | Optional; each value must be `android`, `ios`, or `web`. |
| `conditions.daypartStart` | Optional; must be paired with `daypartEnd`. Format: `HH:MM` (00:00–23:59). |
| `conditions.daypartEnd` | Optional; must be paired with `daypartStart`. Format: `HH:MM` (00:00–23:59). |

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

## Error Response Format

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
| `429 Too Many Requests` | Rate limit exceeded. |
| `500 Internal Server Error` | Unexpected server failures. |
