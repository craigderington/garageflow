# GarageFlow → AWS Lightsail Deployment Design

**Date:** 2026-07-27
**Target:** Debian 13 (trixie), 2 vCPU / 1.9GB RAM / 59GB disk, x86_64, public IP `100.57.235.176`
**Domain:** `garageflow.studio` (nameservers set 2026-07-27, A records not yet created)
**Repo:** `https://github.com/craigderington/garageflow.git`

## Problem

Two findings drive this design. Both were verified, not assumed.

### 1. The web application is not in version control

`apps/web-next` is recorded in the root repo as a gitlink (mode `160000`, commit
`8439afe`) with no `.gitmodules` file. The root repo therefore tracks exactly one
entry under that path — the gitlink — and zero source files:

```
$ git ls-files apps/web-next
apps/web-next
-> tracked entries under apps/web-next: 1
```

The inner repo at `apps/web-next/.git` has a single commit ("Initial commit from
Create Next App", 19 scaffold files). Every route directory (`dashboard`,
`customers`, `estimates`, `repair-orders`, `inspect`, `inspections`, `inventory`,
`labor`, `schedule`, `login`), plus `src/components`, `src/hooks`, `src/lib`, and
the `e2e` suite, is **untracked**.

The entire application exists only as uncommitted working-tree files on one
laptop. This is a data-loss exposure independent of deployment.

### 2. Consequence: the production build cannot succeed

A `git clone` on the server produces an empty `apps/web-next/` directory.
`infra/Dockerfile.web` begins:

```dockerfile
COPY apps/web-next/package.json apps/web-next/package-lock.json ./
```

Since deploys always run `docker compose up -d --build`, this fails at the first
`COPY`. `postgres`, `redis`, `minio`, `api`, and `caddy` would come up normally;
the `web` container — and with it the marketing landing at `/`, which lives at
`apps/web-next/src/app/page.tsx` and ships inside that same container — is the
one service that cannot build.

### Secondary issues

- **No root `.gitignore`.** `.env`, holding live-looking Mailgun, Stripe, and
  Twilio credentials, is untracked but not ignored — one `git add -A` from being
  pushed to GitHub. `apps/web-next/.gitignore` exists (from Create Next App) and
  covers `node_modules`, `.next`, and `.env*`; the root has nothing.
- **No swap.** 1.9GB RAM with Postgres, Redis, and MinIO resident is the profile
  where `next build` is OOM-killed.
- **Apache2 holds `:80`**, blocking Caddy's bind and the ACME HTTP-01 challenge.
- **Dev compose publishes data-service ports** (`5434`, `6379`, `9000`, `9001`).
  On a public IP that exposes Postgres, Redis, and MinIO to the internet.
- **Dev secrets are hardcoded** in `docker-compose.yml`
  (`SESSION_SECRET=dev-secret-change-in-production`, `POSTGRES_PASSWORD=garageflow`,
  MinIO `garageflow123`).
- **`NEXT_PUBLIC_API_URL` is inlined at build time.** It must hold the production
  value before the image is built, not after.

## Design

Five phases. Phase 0 is a prerequisite for all others: without it there is
nothing deployable on the server.

### Phase 0 — Preserve the work (local)

Ordering matters; the ignore rules must land before anything is staged.

1. Write root `.gitignore`: `.env`, `.env.*`, `node_modules/`, `.next/`,
   `test-results/`, `playwright-report/`, `bin/`, `*.log`.
2. Append `test-results/` and `playwright-report/` to `apps/web-next/.gitignore`.
3. Verify `git check-ignore -v .env` reports a match before staging anything.
4. Back up `apps/web-next/.git` to the session scratchpad as a tarball, then
   remove it so the source can be absorbed as normal files. The only history lost
   is the Create Next App scaffold commit.
5. `git rm --cached apps/web-next` to drop the gitlink.
6. Stage everything, then **inspect the staged file list** to confirm no `.env`,
   no `node_modules` (509MB), no build output.
7. Track `docs/` (52KB of engineering specs, currently untracked).
8. Commit and push to `origin/master`.
9. **Gate:** confirm `apps/web-next/src/app/page.tsx` and the route directories
   exist in `origin/master` before touching the server. If this gate fails, the
   deploy cannot proceed.

### Phase 1 — Server preparation

1. 4GB swapfile at `/swapfile`, `vm.swappiness=10`, persisted in `/etc/fstab`.
2. `systemctl disable --now apache2` (purge if not otherwise needed).
3. Verify `docker compose version` (plugin, not legacy `docker-compose`) and that
   `admin` is in the `docker` group.
4. Confirm `:80` and `:443` are free.

### Phase 2 — Production configuration

**Apache runs on the host** and is the reverse proxy: it terminates TLS and
routes to this stack by `Host` header, exactly as every other application on
this server is deployed. GarageFlow ships **no proxy of its own** and binds
**no public interface** — it publishes only to `127.0.0.1`.

Host ports use a dedicated `283xx` block (`28300` web, `28301` api) rather than
the usual `3000`/`8080`, because this server runs several applications and the
common ports collide. They sit below `32768` so they cannot clash with the
kernel's ephemeral range, and are overridable via `WEB_PORT` / `API_PORT`.

- **`docker-compose.prod.yml`**
  - `web` and `api` publish to `127.0.0.1` only, so host Apache can reach them
    and nothing off-box can. Postgres, Redis, and MinIO publish no host port at
    all and stay on the private `internal` network.
  - MinIO is internal-only. The API streams objects to clients itself
    (`storage.Store.Get`), with no presigned URLs anywhere, so no browser ever
    contacts MinIO and it needs no route through Apache.
  - Credentials interpolated from the server's `.env`; no literals.
  - `restart: unless-stopped`, health-gated `depends_on`, and per-service
    memory limits as a runaway guard.
  - Mounts `./migrations` (schema) but **not** `migrations/seed/`.
- **`infra/apache/garageflow.conf`** — installed to
  `/etc/apache2/sites-available/garageflow.studio.conf` and enabled with
  `a2ensite`. Needs `ssl proxy proxy_http proxy_wstunnel headers rewrite`.
  - `ProxyPass` to `127.0.0.1`. Loopback literals always resolve at config-parse
    time, so a stopped backend yields a 502 on this vhost rather than blocking
    Apache from starting.
  - `/api/ws` → `ws://127.0.0.1:28301/ws` via `mod_proxy_wstunnel`, declared
    **before** the `/api/` rule — `ProxyPass` matches in order, and an upgrade
    handshake proxied as plain HTTP fails.
  - `/api/*` → `127.0.0.1:28301` with the prefix stripped, matching the dev
    Caddyfile's `handle_path`. Everything else → `127.0.0.1:28300`.
  - Certificates referenced directly from `/etc/letsencrypt/live/`, so renewal
    needs only an Apache reload — no copy step that can go stale.
  - The `:80` vhost exempts `/.well-known/acme-challenge/` from the HTTPS
    redirect, so http-01 renewal stays available as a fallback.
  - Allows TLS 1.2 alongside 1.3: 1.3-only excludes Android 9 and iOS before
    12.2 — phones that customers open DVI links on.
- **Web build arg:** `NEXT_PUBLIC_API_URL=https://garageflow.studio/api`.
  Same-origin is mandatory, not stylistic: the API's CORS allowlist is hardcoded
  to localhost, so a cross-origin `api.garageflow.studio` would break every
  authenticated request.

### Phase 2a — Security fixes required before exposure

Found while reading the code for the deploy; all are now fixed and verified.

1. **Unauthenticated account takeover.** `POST /auth/magic-link` returned the
   login code in the response body with no dev/prod gate, and
   `GenerateMagicLink` validated only the email's *format*. Two unauthenticated
   requests — magic-link then verify — produced a session for any known address.
   Fixed with an `AUTH_DEV_CODES` flag (default off); the code is echoed only
   when explicitly enabled for dev and E2E.
2. **No rate limiting** on the unauthenticated `/auth/*` endpoints. Added
   `httprate` at `AUTH_RATE_LIMIT_PER_MIN` (default 10/min per IP, 0 disables).
   The E2E suite authenticates far faster than any human, so dev sets 0.
3. **Demo accounts in production.** `002_seed_data.sql` created a shop and three
   users sharing one bcrypt hash of `password123` — a published password on an
   `owner` account. Moved to `migrations/seed/`, which `initdb` ignores because
   it is a subdirectory, so `migrations/*.sql` is now schema only.
   - `005_inspections.sql` also inserted a default template against the demo
     shop's hardcoded UUID. Left alone, production's first boot would have hit a
     foreign-key violation and Postgres would have failed to initialize. That
     insert moved into the seed file.
   - The dev stack must **not** mount a seed file into
     `/docker-entrypoint-initdb.d/` alongside `./migrations`: the target nests
     inside that bind mount, so Docker creates the file on the host in
     `./migrations/`, where it would be committed and then executed in
     production. Dev and E2E apply the seed over psql instead.
4. **No way to log in to a fresh production database.** With the seed gone and
   no signup endpoint, production would have had zero users. Added
   `cmd/createadmin`, built into the API image, which creates a shop, an owner
   with a generated password, and a default inspection template in one
   transaction.

### Phase 3 — Secrets and DNS

- Server-side `.env`, `chmod 600`, never committed: freshly generated
  `SESSION_SECRET` (`openssl rand -hex 32`), strong `POSTGRES_PASSWORD` and MinIO
  credentials, `APP_URL=https://garageflow.studio`.
- DNS A records at the new nameservers: `@` and `api` → `100.57.235.176`
  (and `www` if used). Setting nameservers alone does not create records.

### Phase 4 — Deploy

1. `git pull` on the server.
2. `docker compose -f docker-compose.prod.yml up -d --build`.
3. Run migrations. Note: `migrations/` is currently mounted into
   `/docker-entrypoint-initdb.d`, which Postgres executes **only on an empty data
   directory** — fine for this first deploy, but it is not a migration path for
   subsequent schema changes. `scripts/migrate.sh` is the mechanism going forward.
4. Verify: TLS certificate issued, marketing landing at `/`, a real login, an
   authenticated dashboard request, and one DVI photo upload.

### Phase 5 — Hardening

- Nightly `pg_dump` cron with retention; verify a restore.
- Stripe webhook endpoint → `https://garageflow.studio/api/webhooks/stripe`,
  with `STRIPE_WEBHOOK_SECRET` set so signature verification is active.
- Lightsail firewall: only `22`, `80`, `443`.
- Smoke-test a subset of the Playwright suite against production.

## Assumptions

1. **Integrations start in dev/test mode.** The `.env` contains real-looking
   Stripe, Mailgun, and Twilio values. Live Stripe keys mean real charges from a
   demo environment, so the initial deploy uses test keys and the no-key dev
   senders. Still to confirm before Phase 3.
2. **Postgres runs in-compose**, not Lightsail managed. Simplest on 59GB, and
   backups are handled by the Phase 5 cron rather than by AWS.
3. ~~MinIO needs an HTTPS route for DVI photos.~~ **Resolved:** the API streams
   objects itself and never presigns, so MinIO stays internal-only.

## Known limitations

- **No `/healthz` on the API**, so the `api` container has no HTTP healthcheck
  and uptime monitoring has nothing to poll. Worth adding.
- **CORS origins are hardcoded** to localhost in `main.go`. Harmless while the
  browser calls the API same-origin, but it makes an API subdomain impossible
  without a code change.

## Out of scope

Multi-instance or load-balanced topology, CI/CD on push, staging environment,
CDN, log aggregation, and uptime monitoring. Single-box deploy of the current
stack only.

## Risks

| Risk | Mitigation |
|---|---|
| `next build` OOM on 1.9GB | 4GB swap before any build; build with other services stopped if needed |
| Secrets pushed to GitHub | `.gitignore` written and `check-ignore` verified before first `git add` |
| Wrong `NEXT_PUBLIC_API_URL` baked into image | Set as build arg in prod compose; verified in Phase 4 |
| DNS not propagated when Caddy starts | TLS is the last step; Caddy retries ACME automatically |
| Data-service ports exposed on public IP | No host publishing in prod compose; Lightsail firewall limited to 22/80/443 |
| Losing inner-repo history in Phase 0 | Tarball backup of `apps/web-next/.git` to scratchpad first |
