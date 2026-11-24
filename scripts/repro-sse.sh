#!/usr/bin/env bash
# Conecta em /events, dispara uma rajada de N enqueues e conta os eventos recebidos.
# Esperado por task: 1 evento de "queued" + 1 de "running" + 1 de "completed" (ou "failed"),
# mais um por retentativa. Todos os eventos usam o nome "task".
set -euo pipefail

N="${1:-20}"
# Tempo de espera calculado a partir da duração das tasks e do número de workers.
TASK_DURATION_MS="${TASK_DURATION_MS:-1500}"
WORKERS="${WORKERS:-4}"
UI="${UI:-http://localhost:18080}"
TRIGGER="${TRIGGER:-http://localhost:19090}"
OUT="$(mktemp)"

curl -sN "$UI/events" > "$OUT" &
SSE_PID=$!
sleep 1  # dá tempo para o cliente se inscrever antes da rajada

curl -s -X POST "$TRIGGER/enqueue?n=$N"
BATCHES=$(( (N + WORKERS - 1) / WORKERS ))
WAIT=$(( BATCHES * TASK_DURATION_MS / 1000 + 2 ))
sleep "$WAIT"  # tempo para todas as tasks terminarem

kill "$SSE_PID" 2>/dev/null || true
wait "$SSE_PID" 2>/dev/null || true

RECEIVED=$(grep -c '^event: task' "$OUT" || true)
EXPECTED=$((N * 3))

echo "tasks enfileiradas : $N"
echo "eventos recebidos  : $RECEIVED / $EXPECTED (mínimo esperado)"
echo "arquivo bruto      : $OUT"
