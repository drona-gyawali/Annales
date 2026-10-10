#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE="$ROOT_DIR/.env.integration"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Creating default .env.integration..."
  cat > "$ENV_FILE" << 'ENVEOF'
S3_ENDPOINT=http://127.0.0.1:19000
S3_REGION=us-east-1
S3_BUCKET=annales-integration
AWS_ACCESS_KEY_ID=annales_it_admin
AWS_SECRET_ACCESS_KEY=annales_it_test_password_928374
ENVEOF
  echo ".env.integration created with defaults."
fi

set -a
source "$ENV_FILE"
set +a

docker compose -f compose.integration.yml up -d minio

echo "Waiting for MinIO..."
for attempt in $(seq 1 30); do
  if curl --silent --fail \
    "${S3_ENDPOINT}/minio/health/live" >/dev/null; then
    break
  fi

  if [[ "$attempt" -eq 30 ]]; then
    echo "MinIO did not become healthy."
    docker compose -f compose.integration.yml logs minio
    exit 1
  fi

  sleep 2
done

docker compose -f compose.integration.yml run --rm init-bucket

echo "Running Annales integration tests..."
go test -count=1 -tags=integration -v ./internal/integration
