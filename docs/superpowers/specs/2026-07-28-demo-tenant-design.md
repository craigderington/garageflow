# Demo tenant behind an email capture wall

**Date:** 2026-07-28
**Status:** Approved, ready for implementation planning

## Problem

The login page advertised seeded credentials (`owner@garageflow.app` / `password123`)
in plain text. They did not work in production, because `docker-compose.prod.yml`
deliberately never applies `migrations/seed/` — production must not contain the demo
shop or its published-password owner. The hint was removed in `1ee4868`.

That leaves a real gap. GarageFlow has no signup flow, so a prospect cannot see the
product without someone provisioning an account by hand with `createadmin`. The
product plan's thesis is that the inspection *is* the sales pitch, which means a
prospect needs to run one.

## Goals

- A prospect trades an email address for a working, populated shop
- The demo is interactive: they can create repair orders and complete a DVI
- Captured emails are a usable lead list
- No credentials appear on any public page, ever again

## Non-goals

- Self-service signup for paying customers (separate work)
- Converting a demo shop into a paid account (later; the data model should not
  preclude it, but no conversion flow is in scope)
- Billing, plan selection, or trial expiry mechanics beyond deleting the demo

## Architecture

### Tenancy: per-visitor ephemeral shop

Each captured email provisions its **own** shop, cloned from a template, expiring
after **14 days**.

Rejected alternatives:

- *One shared writable demo shop* — prospects see each other's edits and each
  other's data; anything crude typed in persists until a reset.
- *One shared read-only shop* — a shop-management tool that cannot create a repair
  order or complete an inspection demos badly, and undercuts DVI specifically.

The cost is a provisioning path and an expiry sweep, both small.

### Data model

New migration `migrations/006_demo_tenants.sql`:

```sql
ALTER TABLE shops
    ADD COLUMN is_demo BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN expires_at TIMESTAMPTZ;

CREATE TABLE demo_leads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    -- Nullable and SET NULL, not CASCADE: the lead must outlive its shop. A
    -- prospect whose 14 days lapsed is still a lead, and cascading would delete
    -- the very thing the email wall exists to collect.
    shop_id UUID REFERENCES shops(id) ON DELETE SET NULL,
    return_token TEXT NOT NULL UNIQUE,
    ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ
);

CREATE INDEX idx_shops_expires_at ON shops (expires_at) WHERE expires_at IS NOT NULL;
```

`email` is unique, so a returning prospect resumes their existing shop rather than
accumulating one per visit.

### Identity: the email ambiguity

`users` is `UNIQUE(shop_id, email)`, not globally unique, but
`auth/service.go:100` resolves magic links with:

```sql
SELECT id, shop_id, role FROM users WHERE email = $1
```

With the same email in two shops this returns multiple rows and `QueryRow` silently
takes the first. Today that is a latent bug. A demo makes it routine: any prospect
who later becomes a real user with the same address would be logged into an
arbitrary one of their two shops.

Two responses, both required:

1. **The demo does not use email-based magic links.** The emailed return link is
   `{APP_URL}/demo/resume?token=...`, carrying `demo_leads.return_token` — a
   random opaque value bound to that one lead row and therefore to one shop.
   `GET /demo/resume` looks the token up in `demo_leads`, checks the shop still
   exists and has not expired, issues the session, and stamps `verified_at` and
   `last_seen_at`. It never touches `users.email`, so the ambiguity below cannot
   apply to it. The token lives as long as the shop does — unlike a 15-minute
   magic link, this is a bookmark a prospect may use days later — and dies with
   it, since resume checks `expires_at` on every use.
2. **Fix the underlying query independently.** At minimum make it deterministic
   and explicit about multiple matches. Tracked as separate work, not folded into
   this feature — it affects real accounts today.

The demo shop's owner user still exists (the app needs a `user` row for
`TenantMiddleware`), with a random password that is never displayed or emailed.

### Provisioning

`POST /demo`, unauthenticated, rate-limited by IP with the `httprate` already used
on `/auth`.

Given `{"email": "..."}`:

1. Validate the address (`mail.ParseAddress`, same as `GenerateMagicLink`)
2. If `demo_leads` has this email with a live, unexpired shop → reuse it; skip to 6
3. Create the shop: `is_demo = true`, `expires_at = NOW() + INTERVAL '14 days'`,
   slug derived from a random suffix to avoid collisions
4. Create the owner user with a random password hash
5. Seed template data (below)
6. Insert or update the `demo_leads` row; set `last_seen_at`
7. Issue the session cookie, exactly as `/auth/verify` does
8. Send the return-link email, best effort — a mail failure must not fail the request

Response mirrors the existing auth handlers: `{"status":"ok"}`, no secrets in the
body.

### Template data

A Go package `internal/demo` exposes `Seed(ctx, tx, shopID) error`, inserting a
realistic shop: several customers with vehicles, a few repair orders across
statuses, an in-progress inspection with flagged items, a priced estimate, and
inventory parts including one below minimum stock.

Deliberately **not** reusing `migrations/seed/002_seed_data.sql`. That file must
keep never running in production; coupling to it would undo the protection
`docker-compose.prod.yml` documents. The template is parameterised by `shop_id` and
contains no fixed UUIDs.

### Guardrails

**Outbound is blocked for demo tenants.** A demo shop must never send SMS or email
to a "customer" — a prospect can type any phone number, and the shop's DVI flow
would text a stranger. Implemented by wrapping the configured senders in a decorator
that resolves the request's shop and no-ops when `is_demo`. The UI still reports
"sent" and shows the report link and QR, so the prospect sees the whole flow.

`internal/sms.Sender` and `internal/email.Sender` are already interfaces with
existing no-op implementations, so this is a wrapper, not a rewrite.

**Rate limiting.** `httprate.LimitByIP` on `POST /demo` so nobody mass-provisions
shops. Shop creation is far more expensive than a login attempt, so the limit should
be tighter than `AuthRateLimitPerMin`.

**Expiry sweep.** A ticker goroutine in the API deletes shops where
`is_demo AND expires_at < NOW()`, cascading through the existing foreign keys, and
nulls the lead's `shop_id`. A goroutine rather than asynq: asynq is present but no
worker process runs, so a periodic task would never fire. The sweep shares the
`hubCtx` cancellation already added for the realtime bridge.

### Frontend

**Login page** — a "Try a live demo" affordance opening an inline email field, in
the space where the credentials used to be.

**Marketing landing** — the same entry point. The landing already has a "Start free"
CTA that currently goes to `/login`; this gives it somewhere real to go.

Both post to `/demo` and, on success, navigate to `/dashboard`. The session cookie
is set by the API, so `AuthProvider` picks the user up on mount exactly as it does
after `/auth/verify`.

## Error handling

- Invalid email → 400, inline message, no shop created
- Rate limited → 429, message inviting them to try again shortly
- Provisioning failure → the whole thing runs in one transaction and rolls back;
  no half-seeded shop is ever left behind
- Mail failure → logged, request still succeeds; the prospect is already signed in
- Expired demo, returning visitor → provision a fresh shop and tell them the
  previous one expired

## Testing

**Go unit tests**
- Provisioning is idempotent: the same email twice yields one shop
- An expired demo re-provisions rather than resurrecting
- The demo sender decorator no-ops for `is_demo` and passes through otherwise
- The sweep deletes only expired demo shops, never a real one — the most dangerous
  possible bug in this feature, so it gets an explicit test with a live shop present
- Template seeding produces the expected row counts under a real `shop_id`

**E2E**
- Email capture → lands on a populated dashboard, not an empty one
- A demo shop's DVI send delivers nothing outbound but still shows link and QR
- Resubmitting the same email returns to the same shop
- The rate limit returns 429 rather than provisioning

## Decisions taken

| Decision | Choice |
|---|---|
| Tenancy | Per-visitor ephemeral shop |
| Expiry | 14 days |
| Access | Instant session, plus emailed return link |
| Return link | `demo_leads.return_token` via `GET /demo/resume`, not an email-based magic link |
| Outbound | Blocked entirely for demo tenants |
| Entry points | Login page and marketing landing |
| Expiry mechanism | Ticker goroutine in the API |

## Out of scope, worth tracking separately

- `users WHERE email = $1` returning an arbitrary row across shops — a real bug
  affecting real accounts today, independent of this feature
- Converting a demo shop into a paying account
- Any analytics beyond `created_at` / `verified_at` / `last_seen_at`
