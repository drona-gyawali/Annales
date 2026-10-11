#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

cleanup() {
  echo ""
  echo "Cleaning up..."
  docker compose -f compose.benchmark.yml down -v
}
trap cleanup EXIT

echo "Starting benchmark environment..."
docker compose -f compose.benchmark.yml down -v 2>/dev/null || true
docker compose -f compose.benchmark.yml up -d

echo "Waiting for Annales..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/health >/dev/null; then
    echo "Annales is ready."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "Annales did not become healthy"
    docker compose -f compose.benchmark.yml logs annales
    exit 1
  fi
  sleep 2
done

echo ""
echo "════════════════════════════════════════"
echo "  ANNALES BENCHMARK"
echo "════════════════════════════════════════"
go run ./cmd/benchmark

echo ""
echo "════════════════════════════════════════"
echo "  REDIS BENCHMARK (for comparison)"
echo "════════════════════════════════════════"
echo "Concurrency 10:"
docker run --rm --network host redis:7-alpine \
  redis-benchmark -h 127.0.0.1 -p 6379 -n 10000 -c 10 -t set,get --csv 2>/dev/null | grep -E "SET|GET"

echo "Concurrency 50:"
docker run --rm --network host redis:7-alpine \
  redis-benchmark -h 127.0.0.1 -p 6379 -n 10000 -c 50 -t set,get --csv 2>/dev/null | grep -E "SET|GET"

echo "Concurrency 100:"
docker run --rm --network host redis:7-alpine \
  redis-benchmark -h 127.0.0.1 -p 6379 -n 10000 -c 100 -t set,get --csv 2>/dev/null | grep -E "SET|GET"

echo ""
echo "════════════════════════════════════════"
echo "  TEST C: RECOVERY TIME"
echo "════════════════════════════════════════"
echo "Stopping Annales..."
docker compose -f compose.benchmark.yml stop annales
sleep 1

echo "Starting recovery timer..."
RECOVERY_START=$(date +%s%3N)

BENCH_MODE=recovery go run ./cmd/benchmark &
BENCH_PID=$!

echo "Restarting Annales..."
docker compose -f compose.benchmark.yml start annales

wait $BENCH_PID
