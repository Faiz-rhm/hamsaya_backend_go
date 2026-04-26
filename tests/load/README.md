# Load tests

k6 profiles for the hot read/write paths. Run against a dedicated environment
(staging or a local stack), never production.

## Prerequisites

- [k6](https://k6.io) installed (`brew install k6` on macOS).
- A running backend (`make docker-up && make run` for local).
- A valid access token with verified email status.

Mint a token quickly via `/auth/register`:

```bash
ACCESS_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"load@test.local","password":"Password123!","first_name":"Load","last_name":"Tester","latitude":34.5,"longitude":69.2}' \
  | jq -r '.data.tokens.access_token')
```

## Profiles

| File | Path | Stages | Thresholds |
|---|---|---|---|
| `feed.js` | `GET /api/v1/posts/feed` | ramp 50 VUs over 1m, hold 3m, ramp down 30s | p95 < 400ms, errors < 1% |
| `post_create.js` | `POST /api/v1/posts` | ramp 5 VUs over 30s, hold 2m, ramp down 15s | p95 < 800ms, errors < 5% (429s allowed) |

## Running

```bash
API_URL=http://localhost:8080 ACCESS_TOKEN=$ACCESS_TOKEN \
  k6 run tests/load/feed.js

API_URL=http://localhost:8080 ACCESS_TOKEN=$ACCESS_TOKEN \
  k6 run tests/load/post_create.js
```

## Notes

- **Per-user rate limit (30/hour) caps the write profile.** For a true write
  benchmark, mint a pool of tokens (one per VU) or temporarily remove the
  `LimitPostsCreate()` middleware in the load environment.
- Tokens have a 15-minute TTL — for runs longer than that, refresh them or
  use long-lived test accounts.
- Results are not committed; capture them via `k6 run --out cloud` or
  `--summary-export=summary.json` and store them out-of-band.
