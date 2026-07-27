#!/bin/bash
# Deterministic E2E regression: reset the DB to clean seed state, then run the
# full Playwright suite against the running stack. Pass extra args through to
# playwright (e.g. `bash scripts/e2e.sh customers`).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

bash scripts/db-reset.sh
cd apps/web-next
npm run test:e2e -- "$@"
