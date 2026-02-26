#!/bin/bash

# Simple setup for running integration tests
# This script starts the test database and runs the tests

set -e

echo "Starting test database..."
cd /Users/weilun/Projects/Ark

# Start the test database
docker compose -f docker-compose.yml -f docker-compose.test.yml up postgres-test -d

# Wait for the database to be ready
echo "Waiting for test database to be ready..."
sleep 5

# Check if the database is ready
max_attempts=30
attempt=1
while [ $attempt -le $max_attempts ]; do
    if docker exec ark-postgres-test pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
        echo "Test database is ready!"
        break
    fi
    echo "Waiting for database... (attempt $attempt/$max_attempts)"
    sleep 2
    attempt=$((attempt + 1))
done

if [ $attempt -gt $max_attempts ]; then
    echo "Error: Test database did not become ready in time"
    exit 1
fi

# Run the tests
echo "Running integration tests..."
export TEST_DB_DSN="postgres://postgres:postgres@localhost:5433/ark_test?sslmode=disable"

if [ "$1" = "integration" ]; then
    echo "Running integration tests only..."
    go test ./internal/modules/calendar -run Integration -v
elif [ "$1" = "all" ]; then
    echo "Running all tests..."
    go test ./internal/modules/calendar -v
elif [ "$1" = "bench" ]; then
    echo "Running benchmarks..."
    go test ./internal/modules/calendar -bench=. -benchmem
else
    echo "Running integration tests..."
    go test ./internal/modules/calendar -run Integration -v
fi