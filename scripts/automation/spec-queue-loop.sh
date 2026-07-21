#!/bin/bash
# Spec queue runner — lands ONE spec from docs/plans/queue.txt per invocation.
# Modeled on the proven react-reui-loop.sh pattern (lock via mkdir, logs,
# isolated pi sessions). Outer loop:
#
#   while scripts/automation/spec-queue-loop.sh; do sleep 5; done
#
# Exit 0  = one spec landed, queue has more (or had more) — keep looping.
# Exit 1  = stop: queue empty (done) OR something failed (see QUEUE-STOPPED.md).
set -u

ROOT="${AW_REPO_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"
QUEUE="$ROOT/docs/plans/queue.txt"
LOOP_FILE="$ROOT/scripts/automation/spec-loop.md"
RUN_DIR="$ROOT/.pi-loop"
LOCK_DIR="$RUN_DIR/spec-queue.lock"
STOP_FILE="$ROOT/QUEUE-STOPPED.md"
LANDED_FILE="$RUN_DIR/landed-count"
HEARTBEAT_SECONDS=60

PI="${PI:-pi}"
# Same shape as the react-reui loop; override via env if needed.
PI_ARGS=(${PI_ARGS:---provider github-copilot --model claude-sonnet-4.6 -p --thinking high})

export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
cd "$ROOT" || exit 1
mkdir -p "$RUN_DIR/logs" "$RUN_DIR/sessions"

HEARTBEAT_PID=""
stop_heartbeat() {
  if [ -n "$HEARTBEAT_PID" ]; then
    kill "$HEARTBEAT_PID" 2>/dev/null
    # Reap the job so bash does not print "Terminated" to the terminal.
    wait "$HEARTBEAT_PID" 2>/dev/null
  fi
  HEARTBEAT_PID=""
}

stop_queue() {
  {
    echo "# Spec queue stopped — $(date '+%Y-%m-%d %H:%M:%S')"
    echo
    echo "**Item:** ${SPEC:-<none>}"
    echo "**Reason:** $1"
    echo "**Log:** ${LOG_FILE:-<none>}"
    echo
    echo "Fix the cause (or call the planner agent), delete this file and rerun the loop."
  } > "$STOP_FILE"
  echo "STOPPED: $1"
  exit 1
}

# ── Lock: atomic mkdir + PID liveness check ─────────────────────────────
# Deterministic double-start protection: a second process dies HERE, before
# any LLM call. mkdir is atomic (only one process can create it); the pid
# file inside lets a new run detect a crashed holder instantly instead of
# waiting for a staleness window.
if mkdir "$LOCK_DIR" 2>/dev/null; then
  echo $$ > "$LOCK_DIR/pid"
else
  HOLDER="$(cat "$LOCK_DIR/pid" 2>/dev/null)"
  if [ -n "$HOLDER" ] && kill -0 "$HOLDER" 2>/dev/null; then
    echo "Another queue run is ALIVE (pid $HOLDER). Exiting without dispatching."
    exit 1
  fi
  echo "Stale lock (holder pid ${HOLDER:-unknown} is not running) — taking over."
  rm -rf "$LOCK_DIR"
  mkdir "$LOCK_DIR" || exit 1
  echo $$ > "$LOCK_DIR/pid"
fi
trap 'stop_heartbeat; rm -rf "$LOCK_DIR" 2>/dev/null' EXIT

# ── Preflight ───────────────────────────────────────────────────────────
[ -f "$STOP_FILE" ] && stop_queue "previous stop not acknowledged (QUEUE-STOPPED.md exists)"
[ -s "$QUEUE" ] || { echo "Queue empty — all specs done. 🎉"; exit 1; }

SPEC="$(head -n 1 "$QUEUE")"
[ -f "$SPEC" ] || stop_queue "spec file not found: $SPEC"

if [ -n "$(git status --porcelain)" ]; then
  stop_queue "working tree dirty BEFORE dispatch (ghost session or crashed agent?)"
fi

SPEC_NAME="$(basename "$SPEC" .md)"
LOG_FILE="$RUN_DIR/logs/$(date +%F-%H%M)-$SPEC_NAME.log"
# Scoreboard: total = landed so far + everything still queued (incl. this
# one), so appending new specs mid-run keeps the numbers honest.
LANDED=$(cat "$LANDED_FILE" 2>/dev/null || echo 0)
QUEUED_NOW=$(grep -c . "$QUEUE")
TOTAL=$((LANDED + QUEUED_NOW))
CURRENT=$((LANDED + 1))
echo "=== $(date '+%H:%M:%S') dispatching $SPEC_NAME — task $CURRENT of $TOTAL ==="

# ── Dispatch (with a terminal heartbeat so quiet stretches look alive) ──
STARTED=$(date +%s)
(
  while true; do
    sleep "$HEARTBEAT_SECONDS"
    ELAPSED=$(( ($(date +%s) - STARTED) / 60 ))
    echo "[$(date '+%H:%M:%S')] ⏳ task $CURRENT/$TOTAL ($SPEC_NAME) running for ${ELAPSED}min (log: $LOG_FILE)"
  done
) &
HEARTBEAT_PID=$!

"$PI" --session-dir "$RUN_DIR/sessions" \
  "${PI_ARGS[@]}" \
  "@$LOOP_FILE" "@$SPEC" \
  "Implement this spec completely, following the persistent instructions above. Spec: $SPEC" \
  2>&1 | tee "$LOG_FILE"

stop_heartbeat

# ── Postflight: the agent's word is not enough ──────────────────────────
if [ -n "$(git status --porcelain)" ]; then
  stop_queue "tree dirty AFTER the agent finished (uncommitted work)"
fi
if ! git log -5 --format=%H >/dev/null 2>&1; then
  stop_queue "git log unreadable"
fi
if ! grep -q "Status:.*implemented" "$SPEC"; then
  stop_queue "spec status header not updated to implemented (agent aborted or skipped step 4)"
fi
echo "[$(date '+%H:%M:%S')] --- gates (buildgate + frontend; a few minutes) ---" | tee -a "$LOG_FILE"
if ! go run ./tools/buildgate >>"$LOG_FILE" 2>&1; then
  stop_queue "buildgate red after landing (see log)"
fi
echo "[$(date '+%H:%M:%S')] buildgate green — frontend gate next"
if ! (cd frontend && npm run build:frontend) >>"$LOG_FILE" 2>&1; then
  stop_queue "frontend gate red after landing (see log)"
fi

# wails build regenerates the wailsjs stubs; if that is the ONLY dirt the
# gates produced, commit the regeneration — anything else is a real problem.
if [ -n "$(git status --porcelain)" ]; then
  if [ -z "$(git status --porcelain | grep -v ' frontend/wailsjs/')" ]; then
    git add frontend/wailsjs
    git commit -m "Regenerate wailsjs bindings (queue gates after $SPEC_NAME)" >/dev/null \
      || stop_queue "could not commit regenerated wailsjs bindings"
    echo "[$(date '+%H:%M:%S')] committed regenerated wailsjs bindings"
  else
    stop_queue "gates left unexpected dirt beyond frontend/wailsjs (see git status)"
  fi
fi

# ── Pop the queue, COMMIT the pop (queue.txt is tracked) and report ─────
tail -n +2 "$QUEUE" > "$QUEUE.tmp" && mv "$QUEUE.tmp" "$QUEUE"
git add "$QUEUE"
git commit -m "Queue: pop $SPEC_NAME (landed)" >/dev/null \
  || stop_queue "could not commit the queue pop"
echo "$CURRENT" > "$LANDED_FILE"
REMAINING=$(grep -c . "$QUEUE" || true)
echo "=== $(date '+%H:%M:%S') $SPEC_NAME LANDED — $CURRENT of $TOTAL done, $REMAINING remaining ==="
exit 0
