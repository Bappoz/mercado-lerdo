# Nexora API — contract v1

This contract is what traffic generators, correctness checks and future
scenarios rely on. You may change anything behind it (queries, schema,
infrastructure), but breaking it fails correctness checks.

All bodies are JSON. Errors look like `{"error": "<message>"}`.

## Catalog

### `GET /products?page=<n>&limit=<n>`

Newest products first. `page` ≥ 1 (default 1), `limit` 1–100 (default 20).

```json
{"page": 1, "products": [{"id": 1, "sku": "NX-000001", "name": "…", "category": "…", "price_cents": 1999, "stock": 12, "created_at": "…"}]}
```

### `GET /products/{id}`

One product, including `description`. `404` when it does not exist.

### `GET /search?q=<term>&limit=<n>`

- Returns products whose **name contains `q` anywhere**, case-insensitive.
  `q` is matched literally (`%` and `_` are not wildcards).
- Results are ordered by `created_at`, newest first, then truncated to `limit` (1–100, default 20).
- `q` must have 2–64 characters after trimming spaces, otherwise `400`.

```json
{"query": "lamp", "products": [ … ]}
```

## Accounts

### `POST /auth/login`

```json
{"email": "user00042@nexora.test", "password": "…"}
```

`200` → `{"token": "<opaque>", "expires_in": 86400}`. Wrong email or password → `401`.
Send the token as `Authorization: Bearer <token>` on authenticated routes.

## Orders (authenticated)

### `POST /checkout`

Headers: `Authorization: Bearer <token>`, `Idempotency-Key: <string>` (required).

```json
{"order_ref": "c0ffee-000123", "items": [{"product_id": 42, "quantity": 2}]}
```

- `order_ref`: 8–64 characters, chosen by the client. It identifies the purchase
  end to end and is sent to the payment gateway.
- `items`: 1–20 distinct products, quantity 1–10 each.

| Status | Meaning |
|---|---|
| `201` | Order placed and paid. Body: `{"id", "order_ref", "status", "total_cents", "created_at"}` |
| `400` | Invalid request or missing `Idempotency-Key` |
| `401` | Missing or invalid session |
| `409` | Not enough stock |
| `422` | Unknown product |
| `5xx` | Order not placed |

A `201` means the order is stored **and** the customer was charged exactly once for `order_ref`.

### `GET /orders`

The caller's 50 most recent orders.

## Operations

| Endpoint | Port | Purpose |
|---|---|---|
| `GET /healthz` | 8080 | `200` when the database answers within 1 s |
| `GET /metrics` | 9091 | Prometheus text format: request counts and latency by route, DB pool stats |

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `NEXORA_DATABASE_URL` | — (required) | PostgreSQL connection string |
| `NEXORA_PAYMENT_URL` | `http://payments:8090` | Payment gateway base URL |
| `NEXORA_DB_MAX_CONNS` | `10` | Connection pool size per instance |
| `NEXORA_ADDR` | `:8080` | API listen address |
| `NEXORA_METRICS_ADDR` | `:9091` | Metrics listen address |
