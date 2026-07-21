# Spec — LLM Adapter Streaming with Tool Calls (qualidade/UX, não segurança)

> **Status:** implemented (2026-06-11, commit 1aaf691). Mantida como registro de
> design. Sétima spec da série. **Era de qualidade/UX, não de segurança** —
> diferente das outras da série. Completou a "peça pela metade" do adapter
> OpenAI-compatível: antes, quando havia tools no turno, o streaming caía pro modo
> não-streaming e o usuário ficava com a tela parada até a resposta inteira
> chegar. Mesmos gates do projeto. As fases abaixo são o registro do plano
> executado.
>
> Arquivos-fonte (a verdade atual, ler antes de mexer):
>
> - `internal/infrastructure/agent/openai_compatible.go`:
>   - `generateStream` (≈177): se `len(req.Tools) > 0`, **cai pro `generate()`
>     não-streaming** (linha ≈180, comentário "Streaming of incremental tool-call
>     deltas is not implemented"). Caso texto puro: faz SSE, acumula `Delta.Content`,
>     emite `Partial=true` por delta + um final `TurnComplete=true`.
>   - `generate` (≈300+): caminho não-streaming; parseia `message.ToolCalls`
>     (≈346-364) e devolve `genai.FunctionCall` parts; chama `observeTurn`.
>   - Tipos: `chatCompletionChunk.Delta` (≈91, hoje só lê `Content`),
>     `chatCompletionResponse` com `ToolCalls []oaToolCall` (≈109), `oaToolCall`
>     com `Function.Arguments string` (≈76).
>   - `observeTurn` (≈394) — registra o turno (llm_turns) com `ToolCallsJSON`.
>   - `buildMessages`/`buildRequest` (≈480+) — traduz FunctionCall↔tool_calls.
> - `internal/infrastructure/agent/runtime.go` — consome o `iter.Seq2` (partials +
>   final) e encaminha eventos parciais à UI; só o final entra na sessão.
> - `internal/application/llm_turns.go` — consumidor do turn observer.
> - `openai_compatible_test.go` — cobertura atual (SSE de texto; sem tool-call stream).
> - Relatório da auditoria: este chat (2026-06-11).

## 1. Objective

Implementar **streaming de tool-calls** no adapter OpenAI-compatível, para que
turnos com tools (a maioria dos turnos do agente) também transmitam progresso à UI
em vez de cair no modo não-streaming (tela parada):

1. Acumular os **deltas incrementais de `tool_calls`** do SSE (cada chunk traz
   fragmentos: `index`, `id`, `function.name`, e `function.arguments` em pedaços a
   concatenar) e montar as `FunctionCall` finais.
2. Manter o contrato ADK: emitir parciais (`Partial=true`) enquanto chega texto,
   e um final único (`TurnComplete=true`) com as tool-calls montadas.
3. Manter o **turn logging** (`observeTurn`) e o `finish_reason: tool_calls`
   funcionando nesse caminho.

**Fora de escopo:** mudar o adapter Codex/OAuth (`chatgpt_oauth.go`); mudar o
modelo de tools; reescrever `generate()` não-streaming (vira fallback de segurança).

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **O fallback não-streaming continua existindo** como rede de segurança: se o parsing de deltas de tool-call falhar ou o provedor não suportar, cair em `generate()` (comportamento atual). Não remover. |
| 2 | **Contrato ADK preservado**: parciais com `Partial=true` (texto incremental), final único com `TurnComplete=true` carregando as `FunctionCall` parts montadas. Espelhar o que o caminho de texto puro já faz. |
| 3 | **Montagem de tool-call por `index`**: o formato OpenAI streaming entrega `delta.tool_calls[]` com `index` estável; acumular `id`/`name`/`arguments` por índice e só finalizar no `[DONE]`/`finish_reason`. `arguments` chega como string fragmentada → concatenar e só fazer `json.Unmarshal` no fim. |
| 4 | **`observeTurn` dispara nesse caminho** com o `ToolCallsJSON` montado, igual ao não-streaming — não perder o registro de turno. |
| 5 | **Não emitir FunctionCall parcial** para a UI/sessão: tool-call só vale montada e completa (argumentos podem ser JSON inválido no meio). Texto pode ser parcial; tool-call não. |
| 6 | **Erros do provedor no meio do stream** (`chunk.Error`) continuam abortando com erro propagado, como hoje. |

## 3. What already exists — reuse, don't reinvent

- O caminho de **texto puro** do `generateStream` já implementa o loop SSE, o
  scanner com buffer grande, o tratamento de `[DONE]`, `usage`, `model`,
  `finish_reason` e o yield parcial/final — **reusar a estrutura**, só somar a
  acumulação de tool-calls.
- `generate()` (não-streaming) já parseia `oaToolCall` → `genai.FunctionCall`
  (≈346-364) — **reusar a mesma lógica de montagem** ao finalizar o stream.
- `oaToolCall` e os tipos de request/response já existem — só falta o campo de
  `tool_calls` no `chatCompletionChunk.Delta`.
- `observeTurn`/`usageMetadataFromOpenAI`/`finishReason` — reusar no final.

## 4. Gaps a corrigir (a auditoria)

| # | Gap | Onde | Severidade |
|---|-----|------|-----------|
| G1 | Turno com tools não faz streaming (cai pro não-streaming → tela parada) | `generateStream` ≈180 | Média (UX) |
| G2 | `chatCompletionChunk.Delta` não tem campo `tool_calls` pra acumular | tipos ≈91 | (parte do G1) |
| G3 | Sem teste de SSE com deltas de tool-call | `openai_compatible_test.go` | Média (qualidade) |

## 5. Phases

### Phase 1 — Acumular e montar tool-calls no stream (G1 + G2)

1. Adicionar ao `chatCompletionChunk.Delta` o campo `ToolCalls` no formato delta
   da OpenAI (`index`, `id`, `type`, `function.name`, `function.arguments`).
2. No loop SSE do `generateStream`, quando `req.Tools` estiver presente, **não**
   cair no fallback: acumular por `index` (id/name fixos quando vierem, arguments
   concatenados), continuar emitindo texto parcial se houver, e ao `[DONE]`/
   `finish_reason` montar as `FunctionCall` parts (reusando a lógica do `generate()`).
3. Emitir o final único `TurnComplete=true` com texto (se houver) + as FunctionCall
   parts; chamar `observeTurn` com o `ToolCallsJSON` montado.
4. Manter o fallback `generate()` para falha de parsing/provider sem suporte
   (Decision 1).
5. **Accept:** um turno com tool-call transmite progresso e entrega a(s)
   FunctionCall(s) corretamente montada(s); argumentos JSON reconstituídos do
   fragmento; `observeTurn` registra o turno; texto+tool-call no mesmo turno
   funcionam.

### Phase 2 — Testes (G3)

1. Fixture de stream SSE com deltas de tool-call (incluindo `arguments` quebrado em
   vários chunks e caso texto+tool-call juntos) → asserta FunctionCall montada,
   nome/args corretos, final `TurnComplete`, `observeTurn` chamado.
2. Teste do fallback: parsing inválido → cai em `generate()` sem quebrar.
3. **Accept:** testes verdes cobrindo montagem, fragmentação de arguments,
   texto+tool, e fallback.

### Phase 3 — Documentação

1. Atualizar o comentário do `generateStream` (remover o "not implemented") e
   `docs/SELFCODE.md` (seção LLM adapter) com o novo comportamento e o fallback.
2. **Accept:** docs/comentários refletem o estado real; gate completo verde.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

O teste de streaming de tool-call (Phase 2) é obrigatório e **não depende de rede**
(usar um servidor HTTP de teste / fixture SSE, como o teste de texto atual).

## 7. Risks / attention

- **É qualidade/UX, não segurança** — priorizar depois das specs com gaps de
  segurança reais (#1 vault-memory, #2 chat-memory), mas é a que mais melhora a
  sensação de uso do agente (a maioria dos turnos usa tools).
- **Tool-call só completa** (Decision 5): nunca emitir FunctionCall com argumentos
  JSON pela metade — risco de a sessão/UI receber args inválidos. Montar e validar
  antes de emitir.
- **Formato de delta varia entre provedores** OpenAI-compatíveis (OpenRouter,
  Azure, custom). O fallback (Decision 1) cobre quem não seguir o formato; testar
  com fixture do formato padrão e documentar a premissa.
- **Não perder o turn logging** (Decision 4) — `observeTurn` no caminho novo.
- **Backend só recarrega após restart do app.**
- **Sem `git add -A`** (working copy compartilhada); commits pequenos, um por fase.
  Nomes de arquivo com hífen, nunca travessão.
