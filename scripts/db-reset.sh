#!/bin/bash
# Reset the GarageFlow DB to a clean seeded state (for deterministic E2E regression).
# Truncates all domain tables and re-applies the seed migration.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if [ -f .env ]; then set -a; . ./.env; set +a; fi
DB_URL="${DATABASE_URL:-postgres://garageflow:garageflow@localhost:5434/garageflow?sslmode=disable}"

echo "Resetting database..."
psql "$DB_URL" -q <<'SQL'
TRUNCATE
  audit_logs, labor_logs, estimate_items, estimates, invoices,
  schedules, repair_orders, vehicles, customers, inventory_parts,
  bays, users, shops
RESTART IDENTITY CASCADE;
SQL

# The seed file owns every demo row, including the default inspection template
# (which is cascade-deleted when shops is truncated above). It lives outside
# migrations/ so production never applies it.
for f in migrations/seed/*.sql; do
  echo "  Applying $(basename "$f")..."
  psql "$DB_URL" -f "$f" -q
done
echo "Database reset to seed state."
