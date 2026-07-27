#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Load root .env so every child (migrate.sh, the API in apps/api-go, web) sees the
# same config — notably DATABASE_URL on host port 5434. The API's godotenv.Load()
# runs from apps/api-go and would otherwise miss this file and fall back to :5432.
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi

echo "Starting GarageFlow development environment..."

echo "Starting postgres & redis..."
docker compose up -d postgres redis minio
sleep 3

echo "Running migrations..."
bash scripts/migrate.sh

echo "Starting API server..."
cd apps/api-go && go run ./cmd/server &
API_PID=$!

echo "Starting web dev server..."
cd apps/web-next && npm run dev &
WEB_PID=$!

trap "kill $API_PID $WEB_PID 2>/dev/null" EXIT
echo "GarageFlow running: API on :${PORT:-8080}, Web on :3000"
wait
