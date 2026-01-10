## Overview

`fxrates` is a small Go service/library for ingesting exchange rates from pluggable providers and exposing them over
HTTP / WS (REST + GraphQL).

## What you get

* **Runs as a server out of the box**: start it and query rates via REST/GraphQL.
* **Works as a library**: import it and register your own providers, routes, or integrations.
* **Configurable storage layer**: the storage layer is completely abstracted away, so you can swap implementations
  without touching ingest/providers. Want to use Mongo, or something else? It should be easy.

## Storage implementations

* **PostgreSQL** (production)
* **In-memory** (tests / local dev)

## Providers

Providers are pluggable fetchers (scrapers, APIs, etc.) scheduled by the ingestor/orchestrator and persisted through the
storage interface.

## Quick start

### Run with Postgres

```bash
fxrates serve sql --config ./config.yaml
```

### Run in-memory

```bash
fxrates serve memory --config ./config.yaml
```

## REST API

Base path: `/v1`
All endpoints are read-only.

### Common query params

- `as_of` (optional, RFC3339) - Returns the latest rate at or before this timestamp. Defaults to "now".
- `source` (optional) - Filter by data source (e.g. BCV, different banks, etc).
- `type` (optional) - Filter by rate type: MID, BUY, SELL.
- `limit` (optional) - Page size. Defaults to 100. Clamped to a max (e.g. 500).
- `offset` (optional) - Number of rows to skip. Defaults to 0.

### Data model

Each rate returned looks like:

```json
{
  "as_of": "2026-01-13T04:00:00Z",
  "fetched_at": "2026-01-10T15:43:04Z",
  "base": "USD",
  "target": "VES",
  "rate_type": "MID",
  "source": "BCV",
  "rate": 321.1234
}
```

### Pagination response

Rate endpoints return:

```json
{
  "results": [
    /* exchange rates */
  ],
  "total": 123
}
```

### Errors

Errors are JSON (surprise):

```json
{
  "error": "invalid as_of (must be RFC3339 UTC)"
}
```

### Endpoints

#### `GET /v1/rates/{base}/{target}`

Returns rates for a currency pair, as-of a point in time.

If source/type are omitted, you can get multiple results (one per (source, rate_type) bucket).
Paginated via limit/offset.

Example:

```shell
curl "http://localhost:8080/v1/rates/USD/VES?as_of=2026-01-10T00:00:00Z&limit=100&offset=0"
```

Filter by source/type:

```shell
curl "http://localhost:8080/v1/rates/USD/VES?source=BCV&type=MID"
```

#### `GET /v1/rates/{base}`

Returns rates for a base currency across targets, as-of a point in time.

- If target is not specified (it isn't part of this path), the response can include many targets.
- If source/type are omitted, you can get multiple results per target (one per (source, rate_type) bucket).

Paginated via limit/offset.

Example:

```shell
curl "http://localhost:8080/v1/rates/USD?limit=100&offset=0"
```

Filter:

```shell
curl "http://localhost:8080/v1/rates/USD?source=BCV&type=MID"
```

#### `GET /v1/sources`

Lists distinct sources currently present in storage.

Response:

```shell
{     
  "results": [
        "BBVA Provincial",
        "BCV",
        "Banco Exterior",
        "Banco Nacional de Crédito BNC",
        "Banco Sofitasa",
        "Otras Instituciones",
        "R4"
    ]
}
```

Example:

```shell
curl "http://localhost:8080/v1/sources"
```

#### `GET /v1/currencies`

Lists distinct currencies currently present in storage.

Response:

```shell
{ "results": ["USD", "VES", "EUR"] }
```

Example:

```shell
curl "http://localhost:8080/v1/currencies"
```

### OpenAPI

- Spec: `GET /openapi.yaml`
- UI: `GET /`

## GraphQL

GraphQL is available at:

- Endpoint: `POST /graphql`
- Playground: `GET /playground`

### Query: rates

`rates` mirrors the REST rate endpoints, with optional filters and pagination.

```graphql
query {
    rates(base: "USD", target: "VES", type: MID, source: "BCV", limit: 10, offset: 0) {
        total
        results {
            as_of
            fetched_at
            base
            target
            rate_type
            source
            rate
        }
    }
}
```

You can omit `target`, `source`, and `type` to get all matching buckets:

```graphql
query {
    rates(base: "USD", limit: 50, offset: 0) {
        total
        results {
            target
            source
            rate_type
            rate
            as_of
        }
    }
}
```

### Query: sources / currencies

```graphql
query {
    sources
    currencies
}
```
