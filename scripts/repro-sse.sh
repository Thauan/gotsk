#!/usr/bin/env bash
# Conecta em /events, dispara uma rajada de N enqueues e conta os eventos recebidos.
# Esperado por task: 1 task-added (queued) + 1 task-updated (running) + 1 task-updated (completed/failed).
set -euo pipefail

N="${1:-20}"
UI="${UI:-http://localhost:18080}"
TRIGGER="${TRIGGER:-http://localhost:19090}"
OUT="$(mktemp)"

curl -sN "$UI/events" > "$OUT" &
SSE_PID=$!
sleep 1  # dá tempo para o cliente se inscrever antes da rajada

curl -s -X POST "$TRIGGER/enqueue?n=$N"
sleep 3  # tempo para as tasks terminarem

kill "$SSE_PID" 2>/dev/null || true
wait "$SSE_PID" 2>/dev/null || true

ADDED=$(grep -c '^event: task-added' "$OUT" || true)
UPDATED=$(grep -c '^event: task-updated' "$OUT" || true)
EXPECTED=$((N * 3))
RECEIVED=$((ADDED + UPDATED))

echo "tasks enfileiradas : $N"
echo "task-added         : $ADDED (esperado $N)"
echo "task-updated       : $UPDATED (esperado $((N * 2)))"
echo "eventos recebidos  : $RECEIVED / $EXPECTED"
echo "arquivo bruto      : $OUT"
