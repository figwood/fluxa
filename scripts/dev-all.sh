#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATUS_DIR="$(mktemp -d "${TMPDIR:-/tmp}/fluxa-dev.XXXXXX")"
STATUS_FIFO="$STATUS_DIR/status"

mkfifo "$STATUS_FIFO"
exec 3<>"$STATUS_FIFO"

PIDS=()
NAMES=()
SHUTTING_DOWN=0

cleanup() {
  rm -rf "$STATUS_DIR"
}

stop_all() {
  if [[ "$SHUTTING_DOWN" -eq 1 ]]; then
    return
  fi

  SHUTTING_DOWN=1

  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done

  for pid in "${PIDS[@]}"; do
    wait "$pid" 2>/dev/null || true
  done
}

start_service() {
  local name="$1"
  shift

  echo "Starting $name..."

  (
    set +e
    child_pid=""

    trap 'if [[ -n "$child_pid" ]]; then kill "$child_pid" 2>/dev/null || true; fi' INT TERM

    cd "$ROOT_DIR" || exit 1
    "$@" &
    child_pid=$!

    wait "$child_pid"
    status=$?

    printf '%s:%s\n' "$name" "$status" >&3
    exit "$status"
  ) &

  PIDS+=("$!")
  NAMES+=("$name")
}

trap 'stop_all; cleanup; exit 130' INT
trap 'stop_all; cleanup; exit 143' TERM
trap cleanup EXIT

start_service "api" bash scripts/go.sh run ./apps/api/cmd/api
start_service "worker" bash scripts/go.sh run ./apps/api/cmd/worker
start_service "web" env "BACKEND_BASE_URL=${BACKEND_BASE_URL:-http://localhost:8090}" pnpm --dir apps/web dev

echo
echo "Fluxa services started:"
echo "- api:    http://localhost:8090"
echo "- web:    http://localhost:3090"
echo "- worker: background worker"
echo "Press Ctrl+C to stop all services."
echo

IFS=: read -r exited_name exited_status <&3

if [[ "$exited_status" -eq 0 ]]; then
  echo "[$exited_name] exited"
else
  echo "[$exited_name] exited with code $exited_status" >&2
fi

stop_all
exit "$exited_status"
