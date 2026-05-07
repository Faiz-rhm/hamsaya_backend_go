# Key Rotation Checklist

Every secret listed below has touched disk in plaintext (`.env`, project tree,
or parent directory). Treat them as **compromised** and rotate before the
first production deploy. Do every step — partial rotation defeats the point.

## 1. JWT Secret

- File: `.env` → `JWT_SECRET`
- Why: signs every access/refresh token. If leaked, anyone can forge admin tokens.
- Rotate: `openssl rand -hex 64` → paste into `.env`.
- Side-effect: every existing session is invalidated (good — confirms rotation).
- **Never** reuse the value from `docker-compose.yml`'s historical literal
  (`dev-secret-change-in-production-x`). The compose file no longer hard-codes
  it; the API now fail-fasts on the default.

## 2. MFA Encryption Key

- File: `.env` → `MFA_SECRET_ENCRYPTION_KEY`
- Why: at-rest AES key for stored TOTP secrets.
- Rotate: `openssl rand -hex 32` → paste into `.env`.
- **Migration required**: existing TOTP secrets in DB are encrypted with the
  old key. After rotation, every MFA-enabled user must re-enrol. Alternative:
  decrypt with old key + re-encrypt with new during a maintenance window
  (write a one-shot migration script before changing the env var).

## 3. Resend API Key

- File: `.env` → `RESEND_API_KEY` (currently starts `re_…`)
- Portal: <https://resend.com/api-keys>
- Rotate: revoke the existing key, create a new one, paste into `.env`.
- Side-effect: in-flight email-verification + password-reset tokens still
  work (those are JWT-signed, not API-key-bound).

## 4. Apple Sign-In Private Key (`.p8`)

- Files: `~/.hamsaya-secrets/AuthKey_FRX23MW8TJ.p8` and
  `AuthKey_M4YGKTH4JY.p8` (relocated from project tree)
- Portal: <https://developer.apple.com/account/resources/authkeys/list>
- Rotate: revoke the existing key in the portal, create a new "Sign In with
  Apple" key, download the new `.p8`, store in `~/.hamsaya-secrets/`, update
  `APPLE_KEY_ID` + `APPLE_PRIVATE_KEY` in `.env`.
- **Identity-token verification does not use this key** (it uses Apple's
  public JWKS). The `.p8` only matters if you ever exchange an authorization
  code server-side — irrelevant for the current native iOS flow but rotating
  closes the loop in case server-side flows are added later.

## 5. Firebase Service-Account Key

- File: `~/.hamsaya-secrets/firebase-serviceAccountKey.json` (relocated from
  project root; was at `serviceAccountKey.json`)
- ALSO in: `.env` → `FIREBASE_PROJECT_ID` + `FIREBASE_PRIVATE_KEY` +
  `FIREBASE_CLIENT_EMAIL`
- Portal: GCP Console → IAM & Admin → Service Accounts → select the
  hamsaya-65f00 service account → Keys tab → Delete the existing key, then
  Add Key → Create New (JSON).
- Update `.env` with the new private key (multi-line collapse to single line
  with `\n` escapes — see existing format).

## 6. Google OAuth Client

- File: `.env` → `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET`
- Portal: <https://console.cloud.google.com/apis/credentials>
- Rotate: regenerate the client secret. The client ID stays the same. Update
  `.env`.
- **Now actually checked**: backend validates the token's `aud` claim against
  `GOOGLE_CLIENT_ID`, so this value must match what the mobile app uses.

## 7. Facebook App Secret

- File: `.env` → `FACEBOOK_APP_ID` + `FACEBOOK_APP_SECRET`
- Portal: Meta App Dashboard → Settings → Basic → Reset App Secret
- **Now actually checked**: backend calls `/debug_token` with
  `FACEBOOK_APP_ID|FACEBOOK_APP_SECRET` and verifies the token's `app_id`
  matches.

## 8. MinIO / Storage Access Keys

- File: `.env` → `STORAGE_ACCESS_KEY` + `STORAGE_SECRET_KEY`
- Local dev: hardcoded as `hamsaya-dev-access` /
  `ac0e615dbb2d9b2bdd05c74e90639af5` in `docker-compose.yml`. Acceptable for
  dev (compose now binds 127.0.0.1 only).
- Production: rotate via your S3-compatible storage console, update
  `docker-compose.prod.yml` env or secret manager.

## 9. Database Password

- Dev: `postgres` / `postgres` (acceptable now that the port is bound to
  127.0.0.1).
- Production: use a long random password. Production compose mounts secrets
  at runtime — never commit prod password to repo.

## 10. Redis Password

- File: `.env` + `docker-compose.yml` → `REDIS_PASSWORD`
- Default: `devredispass` (added during this rotation pass). Change for any
  shared environment.

---

## Checklist

```
[ ] JWT_SECRET rotated (openssl rand -hex 64)
[ ] MFA_SECRET_ENCRYPTION_KEY rotated + migration plan or re-enrol
[ ] Resend API key revoked + replaced
[ ] Apple Sign-In .p8 revoked + replaced
[ ] Firebase service-account key revoked + replaced
[ ] Google OAuth client secret regenerated
[ ] Facebook app secret reset
[ ] MinIO access keys rotated (prod only)
[ ] DB password set to a strong value (prod only)
[ ] Redis password set (any shared env)
[ ] git log --all --diff-filter=A -- '*serviceAccount*.json' '*.p8' '.env' returns nothing
[ ] CI logs / artifacts purged of any historical secret print
```

## After rotation

1. Restart backend container so it picks up new env: `docker compose up -d --build api`.
2. Smoke-test: login, OAuth (Google/Facebook/Apple), password reset email,
   FCM push, Apple Sign-In, image + video upload.
3. Verify `/health/ready` reports DB + Redis healthy.
