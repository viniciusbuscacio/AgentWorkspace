# Spec — Logs observability v2 (OTel-lite core, syslog-like UI)

> **Status:** implemented in phases (Phase 1 foundation in `b0f577e`; Phases
> 2-6 completed on top of `main` in this work). This spec defines the logging
> contract after the noisy log set was intentionally zeroed and rebuilt entry
> by entry. All events land in the existing **Logs** module; this is not a new
> module.

## 1. Objective

Rebuild AW logging as useful observability instead of a firehose.

The app should answer these questions from the Logs module:

- What did the app do?
- What did the agent do?
- Why was something blocked?
- Which external content influenced a turn?
- Where did time, tokens, retries, or failures happen?
- What should be investigated after a user reports a bug?

The logging model should be structured enough to support traces, filtering,
and future export, but the UI should stay simple and readable.

## 2. Design posture

Use a hybrid:

- **OpenTelemetry-lite as the internal model.**
  Structured fields, event names, trace/run correlation, span parentage,
  durations, attributes, status, and error details.
- **Syslog-like as the human presentation.**
  Compact rows that read like:

  ```text
  INFO chat.run.completed chat=abc run=def duration=18.4s model=gpt-5
  WARN external.safety.taint_applied channel=browser risk=high run=def
  ERROR tool.call.failed tool=browser.click duration=302ms error="..."
  ```

Do not implement a full OpenTelemetry collector/exporter in v1. Adopt the
data shape and vocabulary first; keep storage local in the vault.

## 3. Non-goals

| Non-goal | Reason |
|---|---|
| New module | Everything belongs in the current Logs module. |
| Per-token streaming logs | Too noisy; token usage belongs on run completion. |
| Full message bodies in logs | Chat history owns message content; logs should reference it. |
| Raw tool results in logs | Tool results may contain secrets, external prompt injection, page text, or files. |
| Raw external content in logs | Store safety metadata, hashes, byte counts, snippets only when explicitly safe. |
| Vendor OTel collector/exporter | Defer until the local model proves useful. |
| Logging every React render or UI route change | Noise; log meaningful actions and failures only. |

## 4. Data model

Keep the existing `logs` table path, but evolve the logical record toward this
shape. Implementation may add columns or encode some fields in `context_json`
as a transition step.

| Field | Required | Notes |
|---|---:|---|
| `id` | yes | Stable log id. |
| `timestamp` | yes | UTC RFC3339Nano. |
| `severity` | yes | OTel-style lowercase: `trace`, `debug`, `info`, `warn`, `error`, `fatal`. |
| `level` | yes | Keep existing syslog numeric compatibility 0-7. |
| `event` | yes | Stable dotted event name, e.g. `chat.run.started`. |
| `message` | yes | Short human message, no secrets. |
| `source` | yes | Broad source: `app`, `chat`, `tool`, `browser`, `security`, `vault`, `provider`, `module`. |
| `module_id` | no | Workspace module id when relevant. |
| `chat_id` / `session_id` | no | Current chat/session id. |
| `run_id` / `trace_id` | no | One user turn or app operation. |
| `span_id` | no | Current sub-operation. |
| `parent_span_id` | no | Parent operation for trace tree. |
| `duration_ms` | no | For completed operations. |
| `status` | no | `ok`, `error`, `blocked`, `canceled`, `denied`, `timeout`. |
| `error_name` | no | Error type/category. |
| `error_message` | no | Scrubbed. |
| `error_stack` | no | Scrubbed, only when useful. |
| `attributes_json` | no | Structured metadata, scrubbed. |
| `created_at` | yes | Insert time. |

### Event naming

Use stable dotted names:

```text
<domain>.<object>.<action>
```

Examples:

```text
app.lifecycle.started
vault.unlocked
module.started
chat.message.sent
chat.run.started
chat.run.completed
chat.run.failed
tool.call.started
tool.call.completed
external.safety.taint_applied
browser.connected
provider.fallback.used
```

Avoid ad-hoc message-only logs like `aw action ok`. The `message` is display
text; `event` is the contract.

## 5. Trace model

Every chat turn should have one `run_id` used as the trace id.

Example tree:

```text
chat.run.started run=R
  chat.message.sent message_id=M
  external.content.distilled channel=browser
  model.call.started provider=openai model=gpt-5
    tool.call.started tool=browser.snapshot
    tool.call.completed tool=browser.snapshot
    external.safety.taint_applied risk=high
    tool.confirmation.requested tool=browser.click
    tool.confirmation.approved tool=browser.click
  model.call.completed tokens.input=... tokens.output=...
chat.run.completed duration_ms=...
```

Non-chat app operations can also create traces:

- app startup
- vault unlock
- browser module startup
- REST/MCP server startup
- Google Workspace auth flow

If there is no natural trace, emit a standalone event with no `run_id`.

## 6. Privacy and safety rules

These are locked decisions.

| # | Rule |
|---|---|
| 1 | Logs never store secrets, tokens, passwords, cookies, auth headers, recovery keys, API keys, or full file/email/page contents. |
| 2 | Chat message content is not duplicated into logs. Log `message_id`, role, length, attachment count, and optional safe preview/hash only if explicitly enabled later. |
| 3 | Tool args/results are not logged raw. Store tool name, call id, duration, status, and small safe metadata. |
| 4 | External content logs record channel, size, risk, detector flags, distillation status, and hashes, not raw content. |
| 5 | Logging is best-effort and must never fail the user operation. |
| 6 | Logs from `logs.*` actions do not recursively log themselves. |
| 7 | Destructive log operations are always user-confirmed in UI and never available as agent write tools by default. |

## 7. Severity mapping

Keep syslog numeric compatibility for the existing module while adding a clear
semantic convention.

| Severity | Syslog level | Use for |
|---|---:|---|
| `fatal` | 2 | unrecoverable app/component failure |
| `error` | 3 | operation failed |
| `warn` | 4 | blocked, degraded, suspicious, fallback, timeout recovered |
| `info` | 6 | meaningful lifecycle/user-visible operation |
| `debug` | 7 | opt-in diagnostic details |
| `trace` | 7 | very detailed opt-in traces, disabled by default |

Do not use `Emergency`/`Alert` labels in normal app logs unless there is a
real process-level catastrophe.

## 8. What to log

### 8.1 App lifecycle

| Event | Severity | Attributes |
|---|---|---|
| `app.lifecycle.started` | info | app version, build, platform, data dir hash |
| `app.lifecycle.ready` | info | startup duration |
| `app.lifecycle.stopping` | info | reason when known |
| `app.lifecycle.stopped` | info | best-effort only |
| `app.lifecycle.panic_recovered` | error/fatal | scrubbed panic type/message |

Shutdown logging is best-effort. It may not fire when the process is killed,
Windows shuts down abruptly, or the app crashes before the hook runs.

### 8.2 Vault

| Event | Severity | Attributes |
|---|---|---|
| `vault.unlocked` | info | profile id/name hash, duration |
| `vault.locked` | info | reason |
| `vault.unlock_failed` | warn | reason category, not password |
| `vault.migration.completed` | info | from/to schema version |
| `vault.migration.failed` | error | migration name, scrubbed error |

### 8.3 Modules

Log module lifecycle only when there is real work or an external resource:

| Event | Severity | Attributes |
|---|---|---|
| `module.started` | info | module id/type, process/resource id |
| `module.stopped` | info | module id/type, reason |
| `module.start_failed` | error | module id/type, scrubbed error |
| `module.health_changed` | warn/info | module id/type, old/new |

Examples: browser, MCP, REST, voice capture, Google Workspace connector.

Do not log passive UI render, tab switch, or sidebar selection.

### 8.4 Chat

| Event | Severity | Attributes |
|---|---|---|
| `chat.message.sent` | info | chat id, run id, message id, char count, attachment count |
| `chat.run.started` | info | chat id, run id, provider, model |
| `chat.run.completed` | info | duration, token usage, tool count, fallback count |
| `chat.run.failed` | error | scrubbed error, provider/model, duration |
| `chat.run.canceled` | info | canceled by user/system, duration |
| `chat.run.queued` | info/debug | queue length, chat id |
| `chat.run.dequeued` | info/debug | queue wait time |
| `chat.compaction.started` | info | auto/manual, message count |
| `chat.compaction.completed` | info | before/after message count, duration |
| `chat.compaction.failed` | warn/error | scrubbed error |
| `provider.fallback.used` | warn | from provider/model, to provider/model, reason category |

Do not log full user or assistant messages. The chat database is the source of
truth for conversation content.

### 8.5 Model calls

| Event | Severity | Attributes |
|---|---|---|
| `model.call.started` | debug/info | provider, model, stream=true/false |
| `model.call.completed` | info | duration, token usage, finish reason |
| `model.call.failed` | error | provider, model, error category |
| `model.context_window.exceeded` | warn | estimated tokens, limit |
| `model.retry.started` | warn/debug | reason, attempt |

### 8.6 Tools and actions

| Event | Severity | Attributes |
|---|---|---|
| `tool.call.started` | debug/info | tool/action name, call id, run id |
| `tool.call.completed` | info | duration, result status, output size only |
| `tool.call.failed` | error | duration, scrubbed error |
| `tool.call.blocked` | warn | policy/safety reason |
| `tool.confirmation.requested` | notice/info | action, risk, timeout |
| `tool.confirmation.approved` | info | action, latency |
| `tool.confirmation.denied` | warn | action, latency |
| `tool.confirmation.timeout` | warn | action, timeout |

Never log raw tool args/results by default.

### 8.7 Security / external content

This is a first-class source because it explains agent behavior.

| Event | Severity | Attributes |
|---|---|---|
| `external.content.received` | debug/info | channel, byte/char count, MIME/format |
| `external.content.distill_started` | debug | channel, distiller id |
| `external.content.distill_completed` | info/debug | duration, input size, output size |
| `external.content.distill_failed` | warn/error | channel, scrubbed error |
| `external.safety.detected` | warn | channel, risk, flags |
| `external.safety.taint_applied` | warn | run id, risk, source channel |
| `external.safety.guard_triggered` | warn | action, risk, confirmation required |
| `external.safety.permission_clamped` | warn | old/new mode, run id |
| `external.safety.bypass_blocked` | warn | reason category |

Channels:

```text
browser, email, file, shell, document, image, attachment, connector, api
```

### 8.8 Browser

| Event | Severity | Attributes |
|---|---|---|
| `browser.connected` | info | browser type, CDP port, profile kind |
| `browser.disconnected` | warn/info | reason |
| `browser.command.started` | debug | command name, target kind |
| `browser.command.completed` | info/debug | duration |
| `browser.command.failed` | error | command, scrubbed error |
| `browser.navigation.started` | info/debug | URL origin only, target tab |
| `browser.navigation.completed` | info/debug | URL origin only, duration |

Do not log full URLs when they may contain tokens/query secrets. Store origin
and redacted path only unless explicitly safe.

### 8.9 Files, documents, attachments

| Event | Severity | Attributes |
|---|---|---|
| `file.read_safe.completed` | info/debug | extension, size, safety risk |
| `document.read_safe.completed` | info | type, pages, chars, duration |
| `visual.read_safe.completed` | info | MIME, dimensions, OCR chars |
| `attachment.processed` | info | count, MIME, size, safety risk |
| `attachment.rejected` | warn | reason: size/MIME/pages/timeout |

Paths should be redacted or reduced to workspace-relative paths where safe.

### 8.10 Integrations

| Event | Severity | Attributes |
|---|---|---|
| `integration.auth.connected` | info | integration id |
| `integration.auth.failed` | warn/error | integration id, reason category |
| `integration.rate_limited` | warn | integration id, retry-after |
| `integration.call.failed` | error | integration id, operation, scrubbed error |

## 9. What not to log by default

Do not re-add these without a specific reason:

- every successful generic `aw action ok`
- every UI refresh/render
- every frontend state update
- every token/chunk streamed
- raw browser DOM/page text
- raw Gmail/email bodies
- raw file contents
- raw shell output
- raw MCP payloads
- full message content
- full URLs with query strings
- duplicated lifecycle spam such as repeated unlock logs without useful context

## 10. UI requirements

The current Logs module remains the home for all of this.

### 10.1 Row view

Rows should render syslog-like, dense and scannable:

```text
19:10:22.123 INFO chat.run.completed chat=abc run=def duration=18.4s model=gpt-5
```

Recommended columns:

- time
- severity badge
- event
- source/module
- compact message
- key attributes

### 10.2 Detail view

Expandable detail should show:

- full structured JSON
- trace/run id
- span id and parent span id
- attributes
- scrubbed error
- related ids: chat, message, tool call, module

### 10.3 Filters

Required filters:

- date
- severity
- source
- event prefix
- chat id/session id
- run/trace id
- module id
- text search
- status (`ok`, `error`, `blocked`, `canceled`, `denied`, `timeout`)
- safety risk

### 10.4 Trace view

Add a trace drawer or grouped view later:

```text
chat.run.started
  chat.message.sent
  model.call.started
    tool.call.started
    tool.call.completed
  model.call.completed
chat.run.completed
```

This can be Phase 2. The row list can ship first as long as `trace_id`,
`span_id`, and `parent_span_id` are stored.

### 10.5 Retention and deletion

Keep both operations:

- **Clean by retention**: keep the last N calendar days.
- **Clear all logs**: delete every current log entry.

Both require explicit confirmation. Agent tools may read logs but must not
clear logs by default.

## 11. Agent-facing access

Keep read-only access first.

### `logs.list`

Filters:

- date
- severity
- source
- event prefix
- chat id/session id
- run id/trace id
- status
- search
- limit, capped

Returned fields should exclude large blobs by default.

### Future `logs.trace`

Input:

- `trace_id` / `run_id`

Output:

- ordered span/event tree
- scrubbed errors
- key attributes

No write/delete agent action in v1.

## 12. Implementation phases

### Phase 0 — Reset policy

Remove noisy log calls and adjust tests that asserted the old firehose.
Keep the storage, module UI, retention, and read action.

Expected result: almost no logs are emitted until Phase 1 explicitly adds
them back.

### Phase 1 — Schema and writer contract

Add or emulate:

- `event`
- `severity`
- `trace_id`
- `span_id`
- `parent_span_id`
- `duration_ms`
- `status`
- structured `attributes_json`

Create one application-level logging API:

```go
logger.Event(ctx, LogEvent{
  Event: "chat.run.completed",
  Severity: "info",
  Source: "chat",
  TraceID: runID,
  SpanID: spanID,
  Duration: duration,
  Status: "ok",
  Attributes: map[string]any{...},
})
```

The API must scrub before write and swallow persistence errors.

### Phase 2 — Core lifecycle logs

Implemented.

Add only:

- app started/ready/stopping best-effort
- vault unlocked/locked
- module started/stopped/failed for real resource modules
- browser connected/disconnected

### Phase 3 — Chat and provider logs

Implemented.

Add:

- message sent metadata
- run started/completed/failed/canceled
- queue/dequeue if useful
- compaction started/completed/failed
- provider fallback
- model call completed/failed with token usage

### Phase 4 — Tool and confirmation logs

Implemented.

Add:

- tool call started/completed/failed/blocked
- confirmation requested/approved/denied/timeout
- sandbox/permission block

### Phase 5 — External safety logs

Implemented.

Add:

- external content received/distilled
- safety detected
- taint applied
- guard triggered
- permission clamp

### Phase 6 — UI upgrades

Implemented.

Add:

- event prefix filter
- trace id filter
- status filter
- risk filter
- syslog-like row formatter
- structured detail panel
- trace tree drawer

## 13. Test requirements

### Backend

- writer scrubs secrets before storing
- writer never fails caller when vault/log store errors
- event names are stable and non-empty
- attributes are structured JSON and scrubbed
- no `logs.*` recursion
- retention uses calendar days
- clear-all deletes every row
- log list caps limits
- trace/span ids are preserved

### Frontend

- Logs module renders syslog-like rows
- filters call service with correct parameters
- detail panel shows structured fields
- retention clean requires confirmation
- clear all requires confirmation
- trace drawer groups parent/child spans once implemented

### Regression

- no raw user message content stored in log rows for `chat.message.sent`
- no raw tool args/results stored for `tool.call.completed`
- no raw external content stored for safety events
- no secret-looking value survives scrubbing

## 14. Example entries

### Chat run completed

```json
{
  "severity": "info",
  "level": 6,
  "event": "chat.run.completed",
  "source": "chat",
  "message": "chat run completed",
  "chat_id": "chat-123",
  "trace_id": "run-456",
  "duration_ms": 18420,
  "status": "ok",
  "attributes": {
    "provider": "openai",
    "model": "gpt-5",
    "tokens.input": 1234,
    "tokens.output": 456,
    "tool.count": 3,
    "fallback.count": 0
  }
}
```

### External safety taint

```json
{
  "severity": "warn",
  "level": 4,
  "event": "external.safety.taint_applied",
  "source": "security",
  "message": "external content tainted the current run",
  "trace_id": "run-456",
  "status": "blocked",
  "attributes": {
    "channel": "browser",
    "risk": "high",
    "flags": ["prompt_injection"],
    "content.sha256": "..."
  }
}
```

### Tool confirmation

```json
{
  "severity": "info",
  "level": 6,
  "event": "tool.confirmation.approved",
  "source": "tool",
  "message": "tool confirmation approved",
  "trace_id": "run-456",
  "span_id": "span-tool-1",
  "status": "ok",
  "attributes": {
    "tool.name": "browser.click",
    "latency_ms": 921
  }
}
```

## 15. Open questions

1. Should `debug` logs be stored by default, or gated by a setting?
2. Should chat message previews ever be logged, or only IDs/counts/hashes?
3. Should traces have a separate table later, or is the log table enough with
   indexes?
4. Should the user be able to export logs as JSONL?
5. Should agent-read `logs.list` hide security-sensitive attributes unless a
   safe debug mode is enabled?

## 16. Gates for implementation phases

Run at the end of each implementation phase:

```sh
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build
```

Implementation must also manually verify the Logs module with:

- empty log table
- retention clean
- clear all
- at least one row per newly added event family
- structured detail view
- no raw secrets or raw external content in expanded JSON
