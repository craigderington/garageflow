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

for f in migrations/002_seed_data.sql; do
  echo "  Applying $(basename "$f")..."
  psql "$DB_URL" -f "$f" -q
done

# The default inspection template (seeded in migration 005) is cascade-deleted
# when shops is truncated above, so re-seed it. Keep in sync with 005.
psql "$DB_URL" -q <<'SQL'
INSERT INTO inspection_templates (shop_id, name, is_default, items) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Courtesy Check',
    true,
    '[
        {"section":"Brakes","label":"Front brake pads"},
        {"section":"Brakes","label":"Rear brake pads"},
        {"section":"Brakes","label":"Brake fluid"},
        {"section":"Tires","label":"Front tire tread"},
        {"section":"Tires","label":"Rear tire tread"},
        {"section":"Tires","label":"Tire pressure"},
        {"section":"Fluids","label":"Engine oil level"},
        {"section":"Fluids","label":"Coolant level"},
        {"section":"Fluids","label":"Washer fluid"},
        {"section":"Battery & Electrical","label":"Battery health"},
        {"section":"Battery & Electrical","label":"Headlights / taillights"},
        {"section":"Under Hood","label":"Serpentine belt"},
        {"section":"Under Hood","label":"Air filter"},
        {"section":"Wipers","label":"Wiper blades"}
    ]'
);
SQL
echo "Database reset to seed state."
