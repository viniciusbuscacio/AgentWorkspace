# Spec - Attachment Taint Stage C

> **Status:** implemented (Option C1 — context-propagated per-turn scope).
> Follow-up to the external-content safety hardening series on branch
> `codex/save-aw-worktree-20260619`.
>
> Stage A is already implemented: user image/PDF attachments are extracted by
> `ports.SafeAttachmentReader`, sanitized, labelled as `UNTRUSTED`, and injected
> into the prompt. This spec defines Stage C: make suspicious/high-risk attachment
> content contaminate the same per-turn taint used by tools, so later sensitive
> actions are gated by enforcement rather than only by model obedience.
>
> Source files to read before implementation:
>
> - `internal/application/chat_run.go` - builds `runtimeText` with
>   `attachmentContext(...)`.
> - `internal/application/chat_attachments.go` - Stage A prompt block and current
>   explicit note that attachments do not feed tool taint yet.
> - `internal/domain/ports/attachment.go` - `SafeAttachmentReader` contract and
>   Stage C comment.
> - `internal/infrastructure/attachmentsafe` - attachment extraction/sanitization
>   adapter.
> - `internal/infrastructure/tools/external_safety.go` - tracked taint,
>   `withTaintScope`, `recordExternalTaint`, action guard.
> - `internal/infrastructure/tools/aw_registry.go` - `tool.Context.InvocationID()`
>   currently scopes tool taint.
> - `internal/infrastructure/tools/tools.go` - sandbox clamp reads current taint.
> - `internal/infrastructure/agent/runtime.go` - ADK runner/session boundary.

## 1. Objective

When a user attaches an image/PDF whose extracted text is suspicious or high
risk, the **same user turn** should be considered tainted for tool enforcement:

1. The attachment text still enters the prompt as sanitized, untrusted data.
2. If the model later tries a sensitive action (`send`, `execute`, `persist`,
   `upload`, `delete`, browser `click`/`fill`, etc.), the existing
   `requireExternalActionGuard` sees the attachment taint and requires
   confirmation.
3. The sandbox permission clamp also applies for the turn, because it already
   consults the same taint.

This closes the remaining gap between external content fetched by tools
(web/email/file/tool output) and external content supplied as chat attachments.

## 2. Locked Decisions

| # | Decision |
|---|----------|
| 1 | **Do not move taint from `InvocationID` to long-lived `SessionID` for the default path.** Session-wide taint is too sticky for normal chats and can create unnecessary friction. |
| 2 | **Stage C should reuse the existing tool taint/enforcement policy.** Do not create a second attachment-only action guard. One policy, one set of confirmations. |
| 3 | **Application remains infrastructure-free.** `internal/application` may use domain ports and domain values, but must not import `internal/infrastructure/tools` or ADK packages. |
| 4 | **Attachment extraction remains safe even without Stage C wiring.** If taint propagation fails or is unavailable, Stage A behavior remains: sanitized, labelled, metadata-capped prompt content. |
| 5 | **Only suspicious or high-risk attachment reads contaminate the turn.** Low-risk extracted text stays labelled untrusted but does not trigger extra enforcement, matching current `ExternalTaint.WithObservation`. |
| 6 | **No raw attachment bytes in the main agent prompt.** Images/PDF bytes are handled by the safe reader/OCR/PDF parser; the main agent receives sanitized text and metadata only. |
| 7 | **No persistent permission mutation.** Any sandbox clamp remains transient for the tainted invocation only. |

## 3. Current State

Stage A currently works like this:

1. `App.runChatMessage` wires `AttachmentReader: a.attachmentReader()`.
2. `RunChatMessage` persists the original user message and attachments.
3. `attachmentContext(ctx, reader, attachments, cfg)` extracts supported
   attachments into `domain.ExternalContentResult`.
4. The prompt receives an `External attachments` block with
   `UNTRUSTED external data`.
5. Unsupported attachments and extraction errors fall back to metadata-only
   `domain.ExternalAttachmentPrompt`.

The gap: `domain.ExternalContentResult` is not passed into
`workspace.recordExternalTaint`, because the application layer does not know the
ADK `InvocationID` used by the tools layer.

## 4. Proposed Design

Use an explicit **turn safety context** that is owned by the composition/runtime
boundary and shared by:

- the application attachment reader path, before the LLM turn runs;
- the tools path, when ADK later invokes `aw` actions in that same turn.

The key idea is to keep Clean Architecture intact by moving the shared contract
into `domain`/`ports`, and letting infrastructure adapters bind it to the
existing tools taint store.

### 4.1 Domain/Port Contract

Add a small port in `internal/domain/ports`, for example:

```go
type ExternalTaintRecorder interface {
    RecordExternalSafety(ctx context.Context, safety domain.ExternalContentSafety)
}
```

Or, if the implementation wants to keep result-level information:

```go
type ExternalTaintRecorder interface {
    RecordExternalContent(ctx context.Context, result domain.ExternalContentResult)
}
```

Rules:

- The port records only domain safety/result values.
- The application layer does not know whether the recorder is backed by tools,
  runtime, logging, or a no-op.
- A nil recorder is valid and preserves Stage A behavior.

### 4.2 Application Wiring

Extend `RunChatMessageInput` with an optional recorder:

```go
AttachmentTaintRecorder ports.ExternalTaintRecorder
```

Then update `attachmentContext` or its caller to record each handled attachment
result after extraction and before prompt injection.

Suggested shape:

```go
res, handled, err := reader.ReadAttachmentText(...)
if handled && recorder != nil {
    recorder.RecordExternalContent(ctx, res)
}
```

The recorder should receive the same `ctx` that will be used to run the chat
turn, so the infrastructure layer can associate attachment taint with the later
tool invocation scope.

### 4.3 Runtime/Invocation Scope

This is the hard part. Today the tools layer gets taint scope from
`tool.Context.InvocationID()` inside `workspace.awDispatch`. Attachment
processing happens earlier, before the ADK tool call exists.

Implement one of these, in order of preference:

#### Option C1 - Runtime-created turn scope, propagated to both attachment and tools

Create a turn scope ID before attachment extraction and place it in `context`.
Make ADK tool dispatch use the same scope when available, falling back to
`tool.Context.InvocationID()`.

Conceptually:

```go
ctx = externaltaint.WithTurnScope(ctx, runIDOrGeneratedTurnID)
```

Then:

- attachment recorder records against that scope;
- `awDispatch` checks context for an existing external taint scope and uses it;
- if absent, `awDispatch` falls back to `ctx.InvocationID()`.

This keeps taint per-turn without switching to session-wide state.

Open question to answer during implementation:

- Does ADK preserve the parent `context.Context` values from `runtime.run(...)`
  into `tool.Context`/tool calls? If yes, C1 is the cleanest path. If not, use
  C2.

#### Option C2 - Run-scoped store keyed by chat run ID

If ADK does not preserve context values, introduce a run scope known to both
`App.runChatMessage` and the tools workspace.

Possible approach:

1. `App.runChatMessage` already creates a `runID`; use it as taint scope.
2. Pass the run scope to the agent runtime context and to the workspace/tools
   adapter through a small shared scope provider.
3. `awDispatch` resolves scope in this order:
   - explicit scope from tool context, if present;
   - active run scope for the chat/run, if available;
   - `tool.Context.InvocationID()` fallback.

C2 requires more wiring but avoids `SessionID` stickiness.

#### Rejected Option B - Session-wide taint

Do not implement as default. It is simpler, but a suspicious attachment can make
a long chat annoying indefinitely. Keep it only as a future "Security High" mode
if product direction explicitly asks for it.

## 5. Phases

### Phase 1 - Port and no-op-safe application integration

1. Add `ports.ExternalTaintRecorder`.
2. Add optional recorder to `RunChatMessageInput`.
3. Update `attachmentContext` or caller so handled attachment results are offered
   to the recorder.
4. Tests:
   - suspicious attachment result is recorded;
   - low-risk attachment result may be offered but does not force application
     behavior;
   - nil recorder preserves current prompt output;
   - unsupported/error attachments are not recorded and still fall back safely.

### Phase 2 - Shared turn scope

1. Introduce a domain/infrastructure-neutral helper for external taint scope
   context, or move the existing tools-only scope helpers to a package that both
   runtime wiring and tools can use without violating architecture.
2. Ensure the chat run creates one scope per user turn.
3. Ensure `awDispatch` uses the shared scope when present and `InvocationID()`
   otherwise.
4. Tests:
   - scope exists before attachment extraction;
   - tool calls in the same turn see the same scope;
   - a second turn gets a different scope.

### Phase 3 - Recorder adapter backed by tools taint

1. Implement the recorder in infrastructure, backed by the existing workspace
   taint store.
2. Convert `domain.ExternalContentResult` to `domain.ExternalContentSafety` and
   reuse the same suspicious/high-risk criteria as `recordExternalTaint`.
3. Wire the recorder in `App.runChatMessage`.
4. Tests:
   - suspicious attachment then clean sensitive action gates confirmation;
   - clean attachment does not gate clean sensitive action;
   - tainted attachment clamps sandbox for that turn;
   - taint does not bleed into the next turn/chat.

### Phase 4 - Documentation and Debuggability

1. Update `docs/SELFCODE.md`: attachments now feed Stage C taint enforcement.
2. Update `internal/domain/ports/attachment.go` comments to remove the "not yet"
   caveat and describe the new recorder path.
3. Add a small log/audit entry when attachment taint is recorded, without logging
   raw attachment text.
4. Consider prompt-debug metadata showing attachment safety summary only
   (`risk`, `suspicious`, `warnings`, `name`, `mime`, size), never raw bytes.

## 6. Acceptance Tests

Minimum required tests:

1. **Application unit:** `attachmentContext` or caller records a suspicious
   attachment result while preserving the untrusted prompt block.
2. **Integration-ish tools/runtime:** suspicious attachment in a user turn causes
   a later clean `memory.remember`/send/persist action in that same turn to
   require confirmation.
3. **No over-gating:** low-risk attachment text does not require confirmation for
   ordinary clean sensitive actions.
4. **Isolation:** taint from attachment in turn A does not affect turn B.
5. **Sandbox clamp:** with tainted attachment, effective sandbox mode is clamped
   to `permit_list` for that turn only.
6. **Fallback:** if attachment reader errors or unsupported MIME is provided, the
   existing metadata-only prompt remains and no taint is recorded.
7. **Architecture:** `internal/architecture` remains green; application does not
   import tools/infrastructure.

## 7. Gates

Run at the end of each phase:

```sh
go test ./internal/application ./internal/domain ./internal/infrastructure/tools
go test ./internal/architecture
go test ./...
go vet ./internal/...
gofmt -l .
```

If frontend files are touched, also run:

```sh
cd frontend && npm run build:frontend
```

## 8. Risks / Attention

- **Context propagation uncertainty:** verify whether ADK preserves parent
  context values into tool execution before committing to C1.
- **Do not create sticky sessions accidentally:** the goal is per-turn taint, not
  long-lived chat taint.
- **Avoid duplicate policy:** attachment taint must feed existing
  `requireExternalActionGuard` and sandbox clamp, not a separate guard with
  different semantics.
- **No secret/raw-text logging:** attachment safety logs must avoid raw extracted
  content because attachments may contain sensitive documents.
- **OCR false positives/negatives:** OCR text may be wrong; it remains external
  data and should still go through `externalsafe`.
- **Concurrent agents/worktree:** avoid broad refactors while the security branch
  is under active review. Keep Stage C in its own PR/commit series.

## 9. Non-goals

- Supporting new file formats beyond current Stage A readers.
- Replacing regex/heuristic detection with a classifier.
- Session-wide taint or a new "Security High" setting.
- Changing user-visible attachment UX unless needed for error reporting.
- Sending raw attachment bytes to the main agent prompt.
