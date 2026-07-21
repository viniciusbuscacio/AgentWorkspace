# Spec — Provider Secrets Hardening (verificação + documentação; baixa prioridade)

> **Status:** implemented (2026-06-11, commit 8a9b7e7; fases obrigatórias feitas,
> Phase 3/refresh do token registrada como follow-up). Mantida como registro de
> design. Sexta spec da série de hardening. **Baixa prioridade** — a auditoria
> (2026-06-11) concluiu que o subsistema de OAuth/providers já estava **sólido**.
> Esta spec **não reescreveu** nada; travou as garantias com testes e documentou,
> mais alguns acertos menores. As fases abaixo são o registro do plano executado.
>
> Arquivos-fonte (a verdade atual):
>
> - `internal/infrastructure/oauth/openai.go` — fluxo browser PKCE (S256, `state`
>   validado, callback `127.0.0.1`, portas fixas 1455/1457 exigidas pela OpenAI,
>   troca HTTPS, `LimitReader`, validação access+refresh).
> - `internal/infrastructure/providers/providers.go` — validação de API key;
>   erro não ecoa a key (só status + body limitado a 240 bytes).
> - `internal/application/provider_config.go` — secrets no vault:
>   keys/credenciais SEM prefixo (`openai_api_key`, `openai_codex_auth_json`, ...),
>   modelo/baseURL/ativo COM `_config_`. `GetProviderStatus` devolve só
>   `connected`/`status`/modelo/placeholders — **nunca o valor da key**.
> - `internal/domain/provider.go` / `llm.go` — `ProviderRuntimeConfig.APIKey` e
>   `.Credential` são `json:"-"` (não serializam). `ProviderSaveConfigInput` tem
>   `apiKey`/`credential` serializáveis, mas é struct de **entrada** (UI→backend).
> - `internal/infrastructure/tools/aw_app.go` — tools do agente: só
>   `provider.status` e `provider.switch`. Nenhum tool de secret.
> - `app.go` — `GetSecret`/`ListSecrets`/`SetSecret`/`DeleteSecret` são bindings
>   Wails (UI), **não** tools do agente.
> - `frontend/src/modules/settings/pages/SecurityPage.tsx` — `isUserManagedSecret`
>   filtra `_`/`chat-`; keys de provider aparecem como credenciais do usuário (por
>   design). `ProvidersPage.tsx` usa `PasswordInput` e nunca revela key salva.
> - `internal/infrastructure/browser/cdp.go:83` — `Runtime.evaluate` é do **Agent
>   Browser** (Chrome separado), não do webview do aw → o agente não chama
>   bindings Wails do aw.
> - Relatório da auditoria: este chat (2026-06-11).

## 1. Objective

O subsistema está sólido; o objetivo aqui é **prevenir regressão** e fechar nits
menores, **sem reescrever** o fluxo:

1. **Travar com testes** que nenhum caminho de *tool do agente* devolve valor de
   secret de provider, e que a config de runtime que carrega a key não serializa.
2. **Documentar** as garantias (vault-only, sem tool de leitura, `json:"-"`,
   webview isolado) para o próximo agente não regredir.
3. **Acertos menores opcionais:** nota sobre portas fixas do OAuth; avaliar/anotar
   o caminho de refresh do token; conferir que `system.state` não despeja valores
   de secret.

**Fora de escopo:** redesenhar OAuth; mudar o armazenamento (vault está certo);
mexer nos tokens MCP/REST (subsistema separado); allowlist de portas.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Nenhum tool do agente pode ler valor de secret/credencial.** O agente tem só `provider.status` (sem valor) e `provider.switch`. Travar com teste; qualquer tool novo que toque secret é proibido por padrão. |
| 2 | **A config de runtime que carrega a key continua `json:"-"`** (`ProviderRuntimeConfig.APIKey`/`.Credential`). Não remover essa tag. |
| 3 | **Secrets de provider continuam no vault criptografado**, nunca em `config.json` nem em log. Erros não ecoam a key (manter o padrão de `providers.go`). |
| 4 | **O webview do aw não expõe `Runtime.evaluate`/eval ao agente** — só o Agent Browser (Chrome separado) tem. Não introduzir eval no webview próprio. |
| 5 | **Portas fixas do OAuth (1455/1457) ficam** — exigência do Hydra da OpenAI; `state`+PKCE são a defesa. Documentar, não "consertar". |
| 6 | **UI nunca revela key salva** — `PasswordInput`, campo nasce vazio, limpo após salvar. Não adicionar "mostrar key" que leia o valor do vault. |

## 3. What already exists — reuse, don't reinvent

Tudo. O fluxo está implementado e correto. O trabalho é **teste + doc**, não código
novo de cripto/OAuth. Reusar `GetProviderStatus` (já mascarado), a tag `json:"-"`,
o `isUserManagedSecret` da UI e o isolamento do webview.

## 4. Gaps a corrigir (a auditoria)

| # | Gap | Onde | Severidade |
|---|-----|------|-----------|
| R1 | Falta teste que prove "nenhum tool do agente devolve valor de secret" (anti-regressão) | tools + provider | Baixa — done |
| R2 | Garantias não documentadas (risco de regressão futura) | `docs/SELFCODE.md` | Baixa — done |
| R3 | Refresh do token OAuth não localizado (robustez, não segurança) | oauth/copilot path | Baixa — located; follow-up |
| R4 | Confirmar que `system.state` não despeja valor de secret | `app_selfdev.go`/state | Baixa — done |

## 5. Phases

### Phase 1 — Testes anti-regressão (R1 + R4)

**Status:** done (2026-06-11).

1. Teste que enumera os tools do agente e falha se algum retornar um valor que
   bata com um secret de provider conhecido (ou, mais simples: asserta que o
   registry de tools não inclui leitura de secret e que `provider.status` não
   contém a key).
2. Teste que serializa `ProviderRuntimeConfig` e confirma que `apiKey`/`credential`
   **não aparecem** no JSON.
3. Conferir/observar `system.state` (selfDevState e afins) e travar com teste que
   não inclui valores de secret.
4. **Accept:** testes verdes provam as três garantias; um PR futuro que exponha
   secret a um tool quebra o teste.

### Phase 2 — Documentação (R2)

**Status:** done (2026-06-11).

1. Em `docs/SELFCODE.md` (seção providers/secrets), registrar: secrets no vault;
   sem tool de leitura; `json:"-"` na runtime config; webview isolado; portas
   fixas do OAuth e o porquê.
2. **Accept:** doc reflete as garantias e o modelo de ameaça (o que protege e o
   que não).

### Phase 3 (opcional) — Refresh do token OAuth (R3)

**Status:** documented follow-up (2026-06-11).

1. Localizar/confirmar onde o access token da OpenAI Subscription é renovado via
   `refresh_token` (ou se hoje força re-login na expiração). Se faltar, anotar como
   item de robustez (não bloqueia esta spec).
2. **Accept:** comportamento de expiração documentado; se houver refresh, coberto
   por teste; se não, registrado como follow-up.

Resultado da localização: `internal/infrastructure/oauth/openai.go` exige e salva
`refresh_token` no payload (`buildCredential`), e
`internal/infrastructure/agent/chatgpt_oauth.go` parseia `RefreshToken`, mas o
request path atual usa apenas `Authorization: Bearer <access_token>`. Não há
refresh automático no runtime; token expirado deve falhar e exigir novo login
por enquanto. Isso é robustez/UX, não gap de segurança.

Follow-up sugerido: implementar renovação no adapter OAuth com persistência do
credential atualizado no vault por uma porta explícita, mantendo o token fora de
logs e respostas de tools.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

## 7. Risks / attention

- **Esta spec é de baixa prioridade** — o subsistema já é seguro. Não deixar ela
  competir com as specs de vault-memory (#1) e chat-memory (#2), que têm gaps reais.
- **Não introduzir leitura de secret por engano** ao escrever os testes (ironia de
  um teste que materializa a key em log). Comparar por presença/ausência, sem
  imprimir valor.
- **Portas fixas do OAuth são imposição do provedor** — não tratar como bug.
- **Backend só recarrega após restart do app.**
- **Sem `git add -A`** (working copy compartilhada); commits pequenos. Nomes com
  hífen, nunca travessão.
