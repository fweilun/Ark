#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

DSN_DEFAULT="postgres://postgres:postgres@localhost:5432/ark?sslmode=disable"
export ARK_TEST_DSN=${ARK_TEST_DSN:-$DSN_DEFAULT}

cd "$ROOT_DIR"

DOCKER_COMPOSE="docker compose"
if ! docker compose version >/dev/null 2>&1; then
  DOCKER_COMPOSE="docker-compose"
fi

echo "Starting Postgres via docker compose..."
$DOCKER_COMPOSE up -d postgres

echo "Waiting for Postgres to be ready..."
for i in {1..30}; do
  if docker exec ark-postgres pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
  if [ "$i" -eq 30 ]; then
    echo "Postgres did not become ready in time" >&2
    exit 1
  fi
done

echo "Running race tests with ARK_TEST_DSN=$ARK_TEST_DSN"
go test -race ./internal/modules/order -run TestConcurrent -count=1
