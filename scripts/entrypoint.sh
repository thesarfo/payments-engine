#!/bin/sh
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required"
  exit 1
fi

echo "Waiting for PostgreSQL..."
until pg_isready -d "$DATABASE_URL" >/dev/null 2>&1; do
  sleep 2
done
echo "PostgreSQL is ready."

echo "Running database migrations..."
migrate_output="$(migrate -path /app/migrations -database "$DATABASE_URL" up 2>&1)" || {
  if echo "$migrate_output" | grep -qi "no change"; then
    echo "$migrate_output"
  else
    echo "$migrate_output"
    exit 1
  fi
}
echo "$migrate_output"

echo "Starting payments engine..."
exec /usr/local/bin/engine
