#!/usr/bin/env python3
"""Dispara uma rajada de tasks e mede a linha do tempo de status de cada uma,
do ponto de vista do cliente SSE (o que o painel recebe, e quando).

Uso: python3 scripts/status-timeline.py [N]
Variáveis: UI (padrão http://localhost:18080), TRIGGER (padrão http://localhost:19090)
"""
import json
import os
import sys
import threading
import time
import urllib.request
from collections import defaultdict

N = int(sys.argv[1]) if len(sys.argv) > 1 else 10
UI = os.environ.get("UI", "http://localhost:18080")
TRIGGER = os.environ.get("TRIGGER", "http://localhost:19090")


events = []  # (t, event_name, task_id, status)
lock = threading.Lock()
connected = threading.Event()


def listen():
    req = urllib.request.Request(f"{UI}/events")
    with urllib.request.urlopen(req) as resp:
        connected.set()
        name = None
        for raw in resp:
            line = raw.decode("utf-8", "replace").rstrip("\n")
            if line.startswith("event:"):
                name = line[len("event:"):].strip()
            elif line.startswith("data:") and name:
                try:
                    msg = json.loads(line[len("data:"):].strip())
                except ValueError:
                    msg = None
                if msg and "id" in msg and "status" in msg:
                    with lock:
                        events.append((time.monotonic(), name, msg["id"], msg["status"]))
                name = None


threading.Thread(target=listen, daemon=True).start()
connected.wait(5)
time.sleep(0.5)

start = time.monotonic()
req = urllib.request.Request(f"{TRIGGER}/enqueue?n={N}", method="POST")
urllib.request.urlopen(req).read()
enqueue_done = time.monotonic() - start

# espera todas as tasks chegarem a um estado final
deadline = time.monotonic() + 60
while time.monotonic() < deadline:
    with lock:
        finals = {e[2] for e in events if e[3] in ("completed", "failed")}
        if len(finals) >= N:
            break
    time.sleep(0.1)

with lock:
    snapshot = list(events)

timeline = defaultdict(list)
for t, name, tid, st in snapshot:
    timeline[tid].append((t - start, st))

print(f"enfileiradas {N} tasks em {enqueue_done*1000:.0f} ms\n")
seen_running = 0
for tid, steps in timeline.items():
    seq = [s for _, s in steps]
    if "running" in seq:
        seen_running += 1
    stamps = "  ".join(f"{s}@{t*1000:.0f}ms" for t, s in steps)
    print(f"{tid[:14]}…  {stamps}")

print()
print(f"tasks que o cliente viu passar por 'running': {seen_running}/{N}")
gaps = []
for steps in timeline.values():
    run = [t for t, s in steps if s == "running"]
    done = [t for t, s in steps if s in ("completed", "failed")]
    if run and done:
        gaps.append((done[0] - run[0]) * 1000)
if gaps:
    print(f"tempo visível em 'running' (ms): min {min(gaps):.0f}  média {sum(gaps)/len(gaps):.0f}  máx {max(gaps):.0f}")
