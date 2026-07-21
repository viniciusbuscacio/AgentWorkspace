# AW Backend Harness

`cmd/awharness` starts a local Agent Workspace backend without opening the Wails UI.
It uses an isolated dev data directory, creates/unlocks a throwaway vault, wires the
`aw` dispatcher, and exposes the REST API.

Default endpoint:

```powershell
.\scripts\run-backend-dev.ps1
```

The script prints the REST URL and bearer token. Defaults are:

- address: `127.0.0.1:9309`
- token: `dev-token`
- data dir: `%TEMP%\aw-harness-data`
- dev profile/vault password: `1234`

Run directly:

```powershell
go run ./cmd/awharness --addr 127.0.0.1:9309 --token dev-token
```

Smoke a one-shot startup:

```powershell
go run ./cmd/awharness --addr 127.0.0.1:0 --once
```

Call an action:

```powershell
$h = @{ Authorization = "Bearer dev-token" }
$body = @{ action = "sandbox.status"; args = @{} } | ConvertTo-Json -Compress
Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:9309/api/aw" -Headers $h -ContentType "application/json" -Body $body
```

## Chat mode (full model-driven turn)

The default harness only exposes the tools layer (`POST /api/aw`). Chat mode adds the
model/chat layer so an external agent can drive a FULL chat turn over REST: the model
decides tools, reads the browser directly via `browser.cdp`, and
returns a final reply.

Chat mode unlocks the user's **REAL** vault (not the throwaway dev vault) so it inherits
the provider + API key already configured in the desktop app. The vault password comes
from the `AW_VAULT_PASSWORD` env var (preferred) or the `-password` flag.

```powershell
$env:AW_VAULT_PASSWORD = "your-real-vault-password"
go run ./cmd/awharness -chat -addr 127.0.0.1:9309 -token dev-token
```

Drive a chat turn:

```bash
curl -s localhost:9309/api/chat -H "Authorization: Bearer dev-token" \
  -d '{"chatId":"t1","text":"liste minhas abas abertas"}'
```

Response shape:

```json
{ "chatId": "t1", "reply": "...final assistant text...", "assistantMessageId": "..." }
```

Notes / limitations:

- `POST /api/chat` is only registered in chat mode; the production app is unaffected.
- Streaming is not implemented yet (`stream:true` is accepted but runs the non-streaming path).
- Requires `AW_VAULT_PASSWORD` (or `-password`) and a provider already configured in the real vault; startup fails clearly otherwise.
- `POST /api/aw` keeps working in chat mode against the same unlocked real vault, so logs/tools/chat share state.

Use this harness for fast backend tests before doing a full Wails build or a manual UI pass.
