# Carehaus AWS Lightsail Deployment Walkthrough

## Summary of Accomplishments

All pre-deployment verification steps for **carehaus** (Go backend, Next.js frontend, Postgres 16 RLS DB, Redis, DocuSeal, and Apache TLS 1.3 reverse proxy) have been completed and validated:

1. **Unit Test Suite**: 100% PASS across 16 packages (`auth`, `billing`, `credential`, `facility`, `medication`, `messaging`, `placement`, `recipient`, `residency`, `scheduling`, `shift`, `shiftclaim`, `shifttemplate`, `tenant`, `respond`, `logging`). Includes automated PHI detection scanner verification.
2. **Schema Migrations**: 100% PASS (all 19 Goose SQL migrations applied cleanly).
3. **Integration & Row-Level Security (RLS)**: 100% PASS (tenant isolation and database policy checks verified on ephemeral Postgres 16).
4. **Production Build**: 100% PASS (built `carehaus-proxy`, `carehaus-api`, `carehaus-worker`, and `carehaus-web` container images).

---

## AWS Lightsail Quick Runbook

To deploy on your AWS Lightsail instance:

```bash
# 1. Provision Lightsail instance (Debian/Ubuntu LTS, 2GB+ RAM)
# 2. Attach Static IP & add A records: curalis.care -> IP, *.curalis.care -> IP
# 3. Install Docker:
curl -fsSL https://get.docker.com | sh && sudo usermod -aG docker "$USER"

# 4. Clone repo and generate secrets:
git clone <repo_url> carehaus && cd carehaus
./scripts/gen-secrets.sh

# 5. Obtain TLS Cert (Certbot DNS-01 for wildcard or ./scripts/gen-certs.sh for staging):
./scripts/gen-certs.sh

# 6. Build and launch database sequence:
docker compose build
docker compose up -d postgres redis
docker compose run --rm migrate
docker compose run --rm provision
docker compose up -d

# 7. Verify health:
curl -fsS https://curalis.care/healthz
```

---

## Key Files Reference
- [CLAUDE.md](file:///home/cd/Work/carehaus/CLAUDE.md): Repository standards and conventions.
- [docs/DEPLOY.md](file:///home/cd/Work/carehaus/docs/DEPLOY.md): Complete deployment documentation.
- [docker-compose.yml](file:///home/cd/Work/carehaus/docker-compose.yml): Production compose definition.
- [scripts/gen-secrets.sh](file:///home/cd/Work/carehaus/scripts/gen-secrets.sh): Cryptographic secrets generator.
- [infra/apache/curalis.conf](file:///home/cd/Work/carehaus/infra/apache/curalis.conf): Apache TLS 1.3 reverse proxy configuration.
