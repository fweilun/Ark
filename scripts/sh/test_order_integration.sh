#!/bin/bash

# README: Integration test runner using Docker Compose
# This script sets up a test environment with PostgreSQL and Redis using Docker Compose
# and runs integration tests for the Order module.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.test.yml"
TEST_MODULE="./internal/modules/order"
COVERAGE_THRESHOLD=70

log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

error() {
    echo -e "${RED}[ERROR] $1${NC}" >&2
}

success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

# Cleanup function
cleanup() {
    log "Cleaning up test environment..."
    docker-compose -f $COMPOSE_FILE down --volumes --remove-orphans || true
}

# Trap cleanup on exit
trap cleanup EXIT

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    error "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    error "docker-compose is not installed or not in PATH"
    exit 1
fi

log "Starting test environment with Docker Compose..."

# Clean up any existing containers
cleanup

# Start services
log "Starting PostgreSQL and Redis services..."
docker-compose -f $COMPOSE_FILE up -d postgres redis

# Wait for services to be healthy
log "Waiting for services to be ready..."
timeout=60
while [ $timeout -gt 0 ]; do
    if docker-compose -f $COMPOSE_FILE ps postgres | grep -q "healthy" && \
       docker-compose -f $COMPOSE_FILE ps redis | grep -q "healthy"; then
        success "Services are ready!"
        break
    fi
    sleep 2
    ((timeout-=2))
done

if [ $timeout -le 0 ]; then
    error "Services failed to start within timeout"
    docker-compose -f $COMPOSE_FILE logs
    exit 1
fi

# Show service status
log "Service status:"
docker-compose -f $COMPOSE_FILE ps

# Test database connection
log "Testing database connection..."
if ! docker-compose -f $COMPOSE_FILE exec -T postgres psql -U postgres -d ark_test -c "SELECT 1;" > /dev/null; then
    error "Failed to connect to test database"
    exit 1
fi
success "Database connection successful"

# Run the integration tests
log "Running Order module integration tests..."

# Set environment variables for the test
export TEST_DB_DSN="postgres://postgres:postgres@localhost:5433/ark_test?sslmode=disable"
export ARK_TEST_DSN="postgres://postgres:postgres@localhost:5433/ark_test?sslmode=disable"
export ARK_REDIS_ADDR="localhost:6380"
export INTEGRATION_TEST="true"

# Run tests with coverage
log "Executing: go test $TEST_MODULE -v -cover -coverprofile=coverage.out -count=1"
if go test $TEST_MODULE -v -cover -coverprofile=coverage.out -count=1; then
    success "All tests passed!"
    
    # Show coverage
    if [ -f coverage.out ]; then
        log "Generating coverage report..."
        go tool cover -func=coverage.out
        
        # Extract coverage percentage
        coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
        
        if [ -n "$coverage" ]; then
            log "Total coverage: ${coverage}%"
            
            # Check coverage threshold
            if (( $(echo "$coverage >= $COVERAGE_THRESHOLD" | bc -l) )); then
                success "Coverage ${coverage}% meets threshold of ${COVERAGE_THRESHOLD}%"
            else
                warn "Coverage ${coverage}% is below threshold of ${COVERAGE_THRESHOLD}%"
            fi
        fi
        
        # Generate HTML coverage report
        go tool cover -html=coverage.out -o coverage.html
        log "HTML coverage report generated: coverage.html"
    fi
    
else
    error "Tests failed!"
    exit 1
fi

success "Integration tests completed successfully!"