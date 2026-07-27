#!/bin/bash
set -euo pipefail

DB_URL="${DATABASE_URL:-postgres://garageflow:garageflow@localhost:5434/garageflow?sslmode=disable}"

echo "Running migrations..."
for f in migrations/*.sql; do
    echo "  Applying $(basename $f)..."
    psql "$DB_URL" -f "$f" -q
done
echo "Migrations complete."
