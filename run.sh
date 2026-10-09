#!/usr/bin/env bash

set -Eeuo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"

PROJECT_NAME="Annales"
API_URL="http://localhost:8080"
MAX_WAIT=120

# Print errors clearly.
die() {
    printf '\nError: %s\n' "$*" >&2
    exit 1
}

# Verify required local tools.
require_tools() {
    command -v docker >/dev/null 2>&1 ||
        die "Docker is not installed. Install Docker Desktop first."

    docker compose version >/dev/null 2>&1 ||
        die "Docker Compose is unavailable. Update Docker Desktop."

    command -v curl >/dev/null 2>&1 ||
        die "curl is required to check API health."
}

# Create credentials only for a fresh installation.
setup_env() {
    if [[ -f .env ]]; then
        printf 'Existing .env found; preserving your configuration.\n'
        return
    fi

    command -v openssl >/dev/null 2>&1 ||
        die "openssl is required to generate secure local credentials."

    local password
    password="$(openssl rand -hex 32)"

    # Restrict access to the new credentials file.
    umask 077

    cat > .env <<ENV
MINIO_IMAGE=pgsty/minio:RELEASE.2026-08-04T00-00-00Z
MINIO_MC_IMAGE=pgsty/mc:latest

MINIO_ROOT_USER=annalesadmin
MINIO_ROOT_PASSWORD=${password}
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
MINIO_BUCKET=logstore-bucket

S3_ENDPOINT=http://localhost:9000
S3_REGION=us-east-1
S3_BUCKET=logstore-bucket

AWS_ACCESS_KEY_ID=annalesadmin
AWS_SECRET_ACCESS_KEY=${password}

ANNALES_ADDR=:8080
ENV

    chmod 600 .env
    printf 'Created .env with fresh local credentials.\n'
}

# Ensure the Docker daemon is running.
check_docker() {
    docker info >/dev/null 2>&1 ||
        die "Docker is not running. Start Docker Desktop and try again."
}

# Build and start Annales plus MinIO.
start_app() {
    require_tools
    setup_env
    check_docker

    printf '\nBuilding and starting %s...\n' "$PROJECT_NAME"
    docker compose up --build -d

    printf '\nWaiting for the API to become healthy'
    local elapsed=0

    until curl --silent --fail "$API_URL/health" >/dev/null 2>&1; do
        if (( elapsed >= MAX_WAIT )); then
            printf '\n\nAnnales did not become healthy in time.\n'
            docker compose ps
            docker compose logs --tail=100 annales minio minio-init
            die "Startup timed out. Check the logs above."
        fi

        printf '.'
        sleep 2
        elapsed=$((elapsed + 2))
    done

    printf '\n\nAnnales is ready!\n\n'
    printf 'API:             %s\n' "$API_URL"
    printf 'MinIO console:   http://localhost:9001\n'
    printf 'Health:          %s/health\n' "$API_URL"
    printf '\nUseful commands:\n'
    printf '  ./run.sh status   Show container status\n'
    printf '  ./run.sh logs     Follow application logs\n'
    printf '  ./run.sh restart  Restart Annales\n'
    printf '  ./run.sh stop     Stop the services\n'
    printf '  ./run.sh down     Remove containers, preserve data\n'
}

case "${1:-up}" in
    up|start)
        start_app
        ;;
    status)
        docker compose ps
        ;;
    logs)
        docker compose logs -f annales
        ;;
    restart)
        docker compose restart annales
        ;;
    stop)
        docker compose stop
        ;;
    down)
        printf 'Removing containers and network; keeping MinIO data volume.\n'
        docker compose down
        ;;
    *)
        printf 'Usage: ./run.sh [up|start|status|logs|restart|stop|down]\n'
        exit 2
        ;;
esac
