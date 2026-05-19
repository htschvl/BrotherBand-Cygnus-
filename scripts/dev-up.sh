#!/usr/bin/env bash
# Start (or stop) the local development services: Postgres + MinIO.
# Usage:
#   ./scripts/dev-up.sh up
#   ./scripts/dev-up.sh down
set -euo pipefail

cd "$(dirname "$0")/.."

ACTION="${1:-up}"

compose() {
  if command -v docker &>/dev/null && docker compose version &>/dev/null; then
    docker compose "$@"
  elif command -v docker-compose &>/dev/null; then
    docker-compose "$@"
  else
    echo "docker compose not found — install Docker Desktop or the compose plugin." >&2
    exit 1
  fi
}

case "$ACTION" in
  up)
    compose -f scripts/docker-compose.dev.yml up -d
    echo "Local services up:"
    echo "  Postgres → localhost:5432   (user/pass: brotherband / brotherband)"
    echo "  MinIO    → localhost:9000   (user/pass: minio / miniominio)"
    echo "  MinIO UI → http://localhost:9001"
    ;;
  down)
    compose -f scripts/docker-compose.dev.yml down -v
    ;;
  *)
    echo "Usage: $0 {up|down}" >&2
    exit 1
    ;;
esac
