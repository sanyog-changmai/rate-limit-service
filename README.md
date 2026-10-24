# Distributed Rate Limiter Service

A standalone distributed rate limiter service built in Go. It acts as a policy enforcement layer that sits between an API Gateway and your backend services. The API Gateway calls this service on every incoming request to decide whether to allow or deny it.

---

## Features

### Core
- Standalone HTTP service — API Gateway calls `POST /check` on every request to get an allow/deny decision
- Token Bucket algorithm with lazy refill — no background goroutines, all math happens at request time inside the Lua script
- Per-client, per-route counters — each `api_key + service + route` combination gets its own independent bucket

### Distributed Correctness
- **Atomic Lua script** — entire read-calculate-write executes as one Redis operation, eliminating race conditions across multiple service instances
- **Redis TIME as clock source** — all instances share the same clock, preventing token miscalculation from clock skew between servers
- **EVALSHA + NOSCRIPT recovery** — SHA hash sent instead of full script on every request; automatic script reload and retry if Redis restarts

### Policy Management
- Per-client rate and burst configured in Postgres (`api_keys` table)
- Two-layer cache: Redis (sub-millisecond) → Postgres fallback on miss — Postgres is never hit on the hot path
- Global algorithm config via `algorithms` table — switch algorithms by flipping a DB row, no code change or restart needed
- Default policy fallback — unknown API keys get a conservative limit instead of an error
- Cache invalidation functions for both policy and algorithm on config change

### Extensibility
- Strategy pattern (`Limiter` interface + factory) — adding a new algorithm requires one new struct and one factory case, nothing else changes
- Algorithm priority ordering — multiple algorithms can coexist in the DB, highest priority wins

### Verified Under Load
- 200 concurrent goroutines tested against a burst=100 bucket — exactly 100 allowed, 0 overshooting, verified with Go's race detector (`-race`)
- Lazy refill accuracy verified — tokens accumulate correctly over time

---

## Setup & Running Locally

### Prerequisites
- Go 1.21+
- Postgres running locally
- Redis running locally

### Environment Variables

Create a `.env` file in the project root:

```env
PORT=8001
DB_URL=host=localhost user=<db_user> password=<db_password> dbname=<db_name> port=5432 sslmode=disable TimeZone=Asia/Kolkata
REDIS_URL=localhost:6379
CACHE_TTL_SECONDS=300
```

### Create the Postgres Database

```sql
CREATE USER <db_user> WITH PASSWORD '<db_password>';
CREATE DATABASE <db_name> OWNER <db_user>;
```

### Run the Service

```bash
go run main.go
```

On startup the service loads `.env`, connects to Postgres, connects to Redis, loads Lua scripts into Redis, runs AutoMigrate, and starts the HTTP server.

### Seed Test Data

```sql
INSERT INTO algorithms (name, is_active, priority) VALUES ('token_bucket', true, 1);

INSERT INTO api_keys (api_key, name, rate, burst, is_active)
VALUES ('test-key-123', 'Test Client', 1.0, 5, true);
```

### Verify

```bash
# health check
curl http://localhost:8001/health

# test rate limiting — burst=5 so 6th request should be denied
for i in $(seq 1 8); do
  curl -s -X POST http://localhost:8001/check \
    -H "Content-Type: application/json" \
    -d '{"api_key":"test-key-123","service":"user-service","route":"/api/users","method":"GET"}'
  echo ""
done

# wait for tokens to refill (rate=1 token/sec)
sleep 3

# this should be allowed again
curl -s -X POST http://localhost:8001/check \
  -H "Content-Type: application/json" \
  -d '{"api_key":"test-key-123","service":"user-service","route":"/api/users","method":"GET"}'
```

---

## Architecture

```
Client (with X-API-Key header)
        ↓
    API Gateway
        ↓  POST /check  { api_key, service, route, method }
  Rate Limiter Service
        ↓
  Step 1: Fetch policy for this API key
          → check Redis cache first (TTL 5 min)
          → on miss: query Postgres, store in Redis
        ↓
  Step 2: Fetch active algorithm from global config
          → algorithms table: highest priority active algorithm wins
        ↓
  Step 3: Run rate limit check
          → Lua script in Redis (atomic — no race conditions)
          → returns allow/deny + remaining tokens
        ↓
  Return { allowed: true/false, remaining: N, retry_after: N }
        ↓
    API Gateway
        ↓ if allowed          ↓ if denied
  Forward to service      Return 429 to client
```

---

## Tech Stack & Packages

| Package | Purpose |
|---|---|
| `github.com/gin-gonic/gin` | HTTP server and routing |
| `gorm.io/gorm` | ORM for Postgres |
| `gorm.io/driver/postgres` | GORM Postgres driver |
| `github.com/joho/godotenv` | Load `.env` file |
| `github.com/google/uuid` | UUID generation for primary keys |
| `github.com/redis/go-redis/v9` | Redis client |

---

## Project Structure

```
rate-limit-service/
├── config/
│   └── config.go                 ← loads .env file
├── internal/
│   ├── dto/
│   │   ├── health_dto.go         ← response shape for /health
│   │   └── check_dto.go          ← request/response shape for /check
│   ├── handler/
│   │   ├── health_handler.go     ← GET /health handler
│   │   └── check_handler.go      ← POST /check handler
│   └── router/
│       ├── router.go             ← registers all routes
│       ├── health.go             ← health route registration
│       └── check.go              ← check route registration
├── cache/
│   └── policy_cache.go           ← Redis cache layer in front of repository queries
├── constants/
│   └── cache.go                  ← cache key prefixes
├── errors/
│   └── errors.go                 ← sentinel errors
├── repository/
│   ├── api_key_repository.go     ← GetPolicyByKey, GetDefaultPolicy queries
│   └── algorithm_repository.go   ← GetActiveAlgorithm query
├── migration/
│   └── migration.go              ← runs GORM AutoMigrate on startup
├── model/
│   ├── api_key.go                ← APIKey model
│   └── algorithm.go              ← Algorithm model
├── pkg/
│   └── response/
│       └── response.go           ← standard JSON response helpers
├── store/
│   ├── postgres.go               ← Postgres connection, global DB var
│   └── redis.go                  ← Redis connection, Lua script loader, global RedisClient + TokenBucketSHA vars
├── limiter/
│   ├── limiter.go                ← Limiter interface + Result struct
│   ├── token_bucket.go           ← token bucket implementation
│   ├── factory.go                ← returns right Limiter based on algorithm name
│   └── scripts/
│       └── token_bucket.lua      ← atomic Lua script for token bucket
├── .env                          ← environment variables (not committed)
├── go.mod
└── go.sum
```

---

## Models

### APIKey (`model/api_key.go`)

One row per client. Stores the client's identity and their rate limit capacity.

```go
type APIKey struct {
    ID        uuid.UUID  // primary key — UUID
    APIKey    string     // the actual key string e.g. "abc123xyz", unique
    Name      string     // human-readable label e.g. "Acme Corp"
    Rate      float64    // token refill rate — tokens per second
    Burst     int        // max bucket capacity — peak burst allowed
    IsActive  bool       // false = key is revoked
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Algorithm (`model/algorithm.go`)

Global algorithm configuration. Not per-client — everyone uses the same algorithm at any given time.

```go
type Algorithm struct {
    ID        uint       // auto-increment primary key
    Name      string     // "token_bucket", "sliding_window" etc.
    IsActive  bool       // only active algorithms are candidates
    Priority  int        // when multiple active, highest priority wins
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## How It Works — Engineering Deep Dive

### Token Bucket Algorithm

Each API key has a virtual bucket. Two numbers define its behavior:
- **Rate** — how fast tokens refill (tokens per second)
- **Burst** — max bucket capacity (max tokens at any point)

Each request consumes 1 token. If the bucket is empty, the request is denied.

```
burst=5, rate=1 token/sec

t=0s:  bucket=5 → request → bucket=4 ✓
t=0s:  bucket=4 → request → bucket=3 ✓
t=0s:  bucket=3 → request → bucket=2 ✓
t=0s:  bucket=2 → request → bucket=1 ✓
t=0s:  bucket=1 → request → bucket=0 ✓
t=0s:  bucket=0 → request → DENIED ✗ (retry_after=1s)

t=3s:  bucket=3 (refilled 3 tokens) → request → bucket=2 ✓
```

**Lazy Refill — No Background Jobs**

Tokens are never added by a background goroutine. Instead, when a request arrives, we calculate how many tokens *would have been added* since the last request:

```
elapsed        = (now_microseconds - last_refill_microseconds) / 1_000_000
tokens_to_add  = elapsed × rate
new_tokens     = min(burst, stored_tokens + tokens_to_add)
```

This avoids needing one goroutine per client. All the math happens at request time inside the Lua script.

**Why Decimals Internally**

Token counts are stored as floats, not integers. At `rate=1/sec`, a request arriving 400ms after the last one earns `0.4` tokens. Storing this fraction means the next request at 700ms later earns `0.3` more — totalling `0.7`. A third request 300ms later earns `0.3` more = `1.0` token = allowed. If we rounded to integers, this accumulated progress would be lost and clients would wait longer than necessary.

We only round when returning `remaining` to the caller — `math.floor()` — because you can't have 0.7 of a request remaining.

---

### Redis — Two Separate Jobs

Redis is used for two completely different purposes in this service:

| Job | Key Pattern | What It Stores | TTL |
|---|---|---|---|
| Policy cache | `config:api_key:{key}` | API key policy from Postgres (rate, burst) | 5 min (configurable) |
| Rate limit counter | `tb:{api_key}:{service}:{route}` | Token bucket state (tokens, last_refill_us) | `(burst/rate) × 2` sec |

These are completely independent. One caches config. One tracks live counters.

**Why Redis and Not Postgres for Counters**

Rate limiting requires a decision in under 1ms on every single request. Postgres takes 10-50ms per query. Redis is in-memory and responds in under 1ms. Also, counters are temporary data — they expire naturally. Redis has built-in TTL. Postgres would need a separate cleanup job.

**Token Bucket State in Redis**

Each client's bucket is stored as a Redis Hash with two fields:

```
Key:    "tb:test-key-123:user-service:/api/users"
Fields: { tokens: 3.5, last_refill_us: 1700000123456789 }
```

`HMGET` and `HMSET` read/write both fields in a single Redis command — one round trip instead of two separate GETs.

---

### Solving the Race Condition Problem

**The Problem**

Without atomicity, two instances of the rate limiter can race on the same bucket:

```
Instance A: reads tokens=1 → calculates allow → writes tokens=0
Instance B: reads tokens=1 → calculates allow → writes tokens=0
Result: 2 requests allowed when only 1 token existed ← WRONG
```

This is a classic TOCTOU (Time of Check, Time of Use) race condition.

**The Fix — Lua Scripts**

Redis is single-threaded. A Lua script executes atomically — no other Redis command from any other client can run between the first and last line of the script. The entire token bucket logic (read → calculate → write) happens in one atomic operation.

```lua
-- All of this runs atomically — nothing can interrupt between these lines
local data        = redis.call('HMGET', KEYS[1], 'tokens', 'last_refill_us')
local new_tokens  = math.min(burst, tokens + elapsed * rate)
if new_tokens < 1 then return {0, 0, burst, retry_after} end
redis.call('HMSET', KEYS[1], 'tokens', new_tokens - 1, 'last_refill_us', now)
```

No race condition is possible.

---

### Solving the Clock Skew Problem

**The Problem**

Token bucket math depends on elapsed time. If two server instances use `time.Now()` from Go and their system clocks differ by even a few milliseconds, they calculate different refill amounts for the same bucket — causing inconsistent behaviour.

**The Fix — Redis TIME**

We call `redis.Time()` before invoking the Lua script to get the current time from Redis's own clock. Since all server instances share the same Redis, they all use the same clock — no skew possible.

```go
timeResult, err := store.RedisClient.Time(ctx).Result()
nowMicros := timeResult.UnixMicro()  // passed to Lua script as ARGV[3]
```

**Why Microseconds**

Redis `TIME` returns `[seconds, microseconds]`. We use microseconds for precision. At high rates (e.g. 100 tokens/sec), millisecond precision causes visible jitter in refills. Microseconds give smooth, accurate behaviour.

---

### Script Loading — EVALSHA vs EVAL

The Lua script is loaded into Redis once at startup using `SCRIPT LOAD`. Redis stores it and returns a 40-character SHA hash.

```go
sha, _ := RedisClient.ScriptLoad(ctx, scriptText).Result()
TokenBucketSHA = sha  // stored globally
```

Every subsequent call uses `EVALSHA` with just the hash — not the full script text:

```go
RedisClient.EvalSha(ctx, store.TokenBucketSHA, keys, args)
```

This avoids sending the full script over the network on every request.

**NOSCRIPT Handling**

If Redis restarts, it loses all loaded scripts. The next `EVALSHA` call returns a `NOSCRIPT` error. We handle this by detecting the error, reloading the script, and retrying once:

```go
if strings.Contains(err.Error(), "NOSCRIPT") {
    store.LoadScripts()  // reload
    res, err = store.RedisClient.EvalSha(...)  // retry
}
```

---

### Two-Layer Policy Lookup

To avoid hitting Postgres on every request:

```
Request arrives with api_key
        ↓
Check Redis: GET "config:api_key:{key}"   ← sub-millisecond
        ↓ hit (most requests)    ↓ miss (first time or TTL expired)
Return cached policy        Query Postgres (~10-50ms)
                                   ↓
                            Store in Redis (TTL = CACHE_TTL_SECONDS)
                                   ↓
                            Return policy
```

Postgres is only hit on cache misses — not on every request.

**Cache Invalidation**

When a policy is updated in Postgres, the Redis cache must be invalidated immediately. Call `cache.InvalidatePolicyCache(apiKey)` after any update. Otherwise stale limits will be enforced for up to `CACHE_TTL_SECONDS`.

---

## Critical Design Decisions

### Why UUID for APIKey primary key and not auto-increment?

1. **Security:** Auto-increment IDs are predictable — if your ID is `42`, someone can guess `41` and `43` exist. UUIDs are random and unguessable.
2. **Distributed safety:** If the DB is ever sharded or multiple write instances run, auto-increment IDs can collide. UUIDs are guaranteed unique without coordination.

### Why is algorithm global and not per-client?

The algorithm is an infrastructure decision — it affects how the entire system behaves. Capacity differences (rate, burst) already handle tier differentiation. Premium customers get higher numbers, not a different algorithm.

### Cache Invalidation on Policy Update

When an API key's policy is updated in Postgres, the Redis cache must be invalidated immediately — otherwise requests will use the stale cached policy for up to 5 minutes. Call `cache.InvalidatePolicyCache(apiKey)` after any update. Same applies for algorithm changes — call `cache.InvalidateAlgorithmCache()`.

For testing: manually delete the cache key with `redis-cli DEL config:api_key:{your_key}`.

### Why API key and not IP or User ID for client identification?

- **IP** breaks behind proxies and NATs — an entire company can share one IP.
- **User ID** requires JWT decoding which needs the auth secret — a separate service shouldn't own that.
- **API key** is clean, explicit, and per-client. Each customer gets their own key with their own limits.

---

## API Reference

### POST /check

Called by the API Gateway on every request to check if it should be allowed through.

**Request:**
```json
{
  "api_key": "abc123xyz",
  "service": "user-service",
  "route": "/api/users",
  "method": "GET"
}
```

**Response (allowed):**
```json
{
  "allowed": true,
  "remaining": 47,
  "limit": 50,
  "reset_at": 1700000180
}
```

**Response (denied):**
```json
{
  "allowed": false,
  "remaining": 0,
  "limit": 50,
  "reset_at": 1700000180,
  "retry_after": 3
}
```

---

### GET /health

Returns server and database status.

**Response (healthy):**
```json
{
  "success": true,
  "message": "Server is up and running.",
  "data": {
    "message": "All services are up and running.",
    "timestamp": "Sat, 24 Oct 2026 12:00:00 IST",
    "db_status": "up",
    "redis_status": "up"
  }
}
```

**Response (degraded):**
```json
{
  "success": false,
  "message": "One or more services are down.",
  "error": "health check failed"
}
```

---

## Tests

Integration tests live in `test/` and require Redis running locally.

```bash
go test ./test/... -v -race -count=1
```

### What is tested

**`TestConcurrentBurst`** — Spawns 200 goroutines simultaneously against a single bucket with `burst=100`. Asserts exactly 100 requests are allowed and 100 are denied, every time. Run with `-race` to verify no data races.

This test proves the Lua script's atomicity is working correctly. Because the entire read-calculate-write happens as one atomic Redis operation, 200 concurrent goroutines can never cause the bucket to allow more than `burst` requests — no matter how they are scheduled. Without the Lua script (plain HMGET + HMSET), this test would intermittently allow more than 100.

**`TestTokenRefill`** — Drains a bucket of 5 tokens, confirms the next request is denied, waits 3 seconds, then confirms exactly 3 tokens have refilled and are granted. Verifies the lazy refill math is accurate.

### Results

```
=== RUN   TestConcurrentBurst
--- PASS: TestConcurrentBurst (0.02s)
=== RUN   TestTokenRefill
--- PASS: TestTokenRefill (3.00s)
PASS
ok      rate-limit-service/test    4.707s
```
