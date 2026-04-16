#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DB_HOST="${DB_HOST:-192.168.0.132}"
DB_PORT="${DB_PORT:-5431}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-128714}"
DB_NAME="${DB_NAME:-framestack}"
DB_SSLMODE="${DB_SSLMODE:-disable}"
HTTP_ADDR="${HTTP_ADDR:-:8080}"
WORKER_PROVIDER="${WORKER_PROVIDER:-lm_studio}"
WORKER_PROVIDER_TIMEOUT_SEC="${WORKER_PROVIDER_TIMEOUT_SEC:-600}"

MODE="${1:-dev}"

export IDEA_HTTP_ADDR="$HTTP_ADDR"
export IDEA_DATABASE_URL="${IDEA_DATABASE_URL:-postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}}"
export IDEA_ENABLE_DATABASE=true
export IDEA_DEFAULT_PROVIDER="$WORKER_PROVIDER"
export IDEA_WORKER_PROVIDER="$WORKER_PROVIDER"
export IDEA_WORKER_PROVIDER_TIMEOUT_SEC="$WORKER_PROVIDER_TIMEOUT_SEC"

case "$MODE" in
  migrate)
    export IDEA_RUN_MIGRATIONS=true
    export IDEA_RUN_JOB_WORKER=false
    ;;
  dev)
    export IDEA_RUN_MIGRATIONS=false
    export IDEA_RUN_JOB_WORKER=true
    ;;
  *)
    echo "Usage: $0 [migrate|dev]"
    echo "Examples:"
    echo "  $0 migrate"
    echo "  $0 dev"
    echo "  DB_NAME=mydb $0 migrate"
    exit 1
    ;;
esac

echo "Starting FrameStack"
echo "  mode: $MODE"
echo "  http: $IDEA_HTTP_ADDR"
echo "  db:   $IDEA_DATABASE_URL"
echo "  worker provider: $IDEA_WORKER_PROVIDER"
echo "  worker timeout: ${IDEA_WORKER_PROVIDER_TIMEOUT_SEC}s"

cd "$ROOT_DIR"
exec go run ./cmd/server
