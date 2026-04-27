# ADR-001 — Deferred Debt (post P3 sweep)

Date: 2026-04-27

## Context

P3 debt sweep (2026-04 session) shipped:

- **MFA at-rest encryption** — AES-256-GCM, see `pkg/crypto/secret_cipher.go`. Magic-prefixed envelope with backwards-compat for legacy plaintext rows. Key from `MFA_SECRET_ENCRYPTION_KEY`.
- **Read-replica router foundation** — `DB.ReplicaPool` + `DB.Reader()`. Opt-in via `DB_REPLICA_HOST`. Failure non-fatal (replica-down does not block boot).
- **WS hub sharding** — 16 shards, `fnv32a(userID) % 16` routing. Each shard runs its own select loop. Removes single-broadcaster bottleneck.
- **WebP transcode foundation** — `pkg/transcode/queue.go` + `pkg/transcode/worker.go`. Redis-backed BLPUSH/BRPOP with dead-letter list.

The following items are explicitly deferred. Each has a concrete migration plan.

---

## 1. Wire WebP transcode pool into upload handlers

### Status
Deferred. Foundation shipped; no upload handler currently calls `Queue.Enqueue`.

### Why deferred
Mobile clients today expect the upload response to include the final WebP URL. Wiring async transcode requires either:

(a) Server-side: keep the URL stable but serve "original" bytes until the transcode lands, then transparently swap. Needs Cache-Control + ETag handling that doesn't surprise CDN.

(b) Mobile-side: handle 404-during-transcode with retry-on-fetch. Needs a coordinated mobile + backend release.

Either path is multi-day with non-trivial CDN / mobile coordination.

### Migration plan
1. Backend: add a `Storage.Transcode` method that satisfies `transcode.Encoder` (fetches MinIO key, encodes, writes target key).
2. Backend: in `cmd/server/main.go`, when `TRANSCODE_ASYNC=true`, start a `transcode.Pool` with N workers.
3. Backend: in `internal/handlers/post_handler.go` upload paths, write original to `<key>.orig`, enqueue transcode → `<key>.webp`, return `<key>.webp` as URL.
4. Mobile: bump retry-on-404 budget to ~30s for newly uploaded post images.
5. Roll out behind `TRANSCODE_ASYNC` env flag; flip after 1 week of clean dual-write.

### Effort
~6 hr backend + ~2 hr mobile + 1 week soak.

---

## 2. External secrets manager (Vault / SSM)

### Status
Deferred. All secrets read from env vars today.

### Why deferred
Genuine deployment-infra task. Needs:
- Choice of provider (HashiCorp Vault vs AWS SSM Parameter Store vs Doppler vs 1Password Connect).
- Auth flow (Kubernetes ServiceAccount → Vault role, or IRSA → SSM IAM).
- Lease renewal / cache invalidation if rotation is automatic.
- Bootstrap secret (Vault token, or AWS credentials) — itself needs secure delivery.

Cannot ship this from a code session alone — it's a deploy-pipeline change.

### Migration plan
1. Pick provider. Recommendation: **AWS SSM Parameter Store** if hosting on EKS/EC2 (free, integrated, no extra infra). Vault otherwise.
2. Add a thin `SecretSource` interface in `pkg/secrets/`:
   ```go
   type SecretSource interface {
       Get(ctx context.Context, key string) (string, error)
   }
   ```
3. Implementations: `EnvSource` (current behavior), `SSMSource`, `VaultSource`.
4. `config.Load()` becomes async, accepts a `SecretSource`. Hot secrets (`JWT_SECRET`, `MFA_SECRET_ENCRYPTION_KEY`, `DB_PASSWORD`, `RESEND_API_KEY`, OAuth client secrets) flow through `SecretSource.Get`.
5. Source chosen by env: `SECRETS_BACKEND=env|ssm|vault`.
6. CI deploys configure the chosen backend's policies / IAM.

### Effort
~1 day code + 2-3 days infra.

---

## 3. Profile snake_case → camelCase mobile

### Status
Deferred. Affects 56 files.

### Why deferred
Lint `non_constant_identifier_names` is info-level. Renaming `is_blocked` → `isBlocked`, `first_name` → `firstName`, etc. across freezed-generated `Profile` and `User` models would touch 56 files of feature code. High risk for low value during the pre-prod stabilization window.

### Migration plan
1. Add `@JsonKey(name: 'is_blocked')` annotations on each freezed field, rename Dart-side identifiers to camelCase.
2. Regenerate freezed + json_serializable.
3. Codemod feature code: `dart fix --apply` for the rename, manual cleanup for any cases the codemod misses.
4. Run full app smoke test, verify no JSON deserialization regressions.

Best done as a single dedicated PR after a release cut.

### Effort
~3 hr mechanical + 1 hr smoke testing.

---

## 4. WS hub Redis pub/sub for multi-instance scale-out

### Status
Single-instance sharding shipped. Multi-instance fanout deferred.

### Why deferred
Sharding fixes the single-broadcaster goroutine bottleneck within one process. Going wider (multi-pod) requires Redis pub/sub or NATS fanout so a message originated on pod A can reach a client connected to pod B.

### Migration plan
1. Add Redis pub/sub publisher to `Hub.SendToUser` — publishes on `ws:user:<userID>`.
2. Each pod subscribes on `ws:user:*` (pattern subscribe).
3. On pub message receipt, route to local shard (which checks if the userID is locally connected; no-op if not).
4. Add a small TTL'd "user routing" set in Redis to avoid pubsub storms (only one pod publishes; others stay silent).

### Effort
~4-6 hr.
