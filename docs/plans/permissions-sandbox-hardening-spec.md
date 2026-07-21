# Spec — Permissions Sandbox Hardening (shell no Windows + self-dev)

> **Status:** implemented (2026-06-11, commit ab218ce). Mantida como registro de
> design. Mesma série e convenções das specs anteriores (`permissions-spec.md`,
> `workspace-modules-spec.md`, `ax-tree-hardening-spec.md`,
> `agent-browser-hardening-spec.md`) — mesmos gates. Fechou os gaps da auditoria
> de segurança da cerca de permissões (2026-06-11). **Não** redesenhou o modelo
> de 4 modos nem o chokepoint único; endureceu a extração de paths do shell
> (antes Unix-only) e documentou explicitamente a postura do self-dev. As fases
> abaixo são o registro do plano executado.
>
> Arquivos-fonte (a verdade atual, ler antes de mexer):
>
> - `internal/infrastructure/sandbox/extract.go` — `ExtractPaths`, `tokenize`,
>   `looksLikePath`, `IsSafeNoPathCommand`, `CheckCommand`, `unsafeOperators`.
> - `internal/infrastructure/sandbox/checker.go` — `IsPathAllowed`, `ExpandPath`,
>   `normalizeForCompare`, `isInsideFolder`, built-in lists.
> - `internal/infrastructure/sandbox/resolve.go` — `ResolveAndCheck` (defesa de symlink).
> - `internal/domain/sandbox.go` — modos, `SandboxConfig`, `DefaultSandboxConfig`,
>   `sandboxBuiltinDenied`, `sandboxBuiltinAllowedPermitList`.
> - `internal/infrastructure/tools/tools.go` — `resolve()`, `runShell()`,
>   `sandboxPolicy()`, chokepoints de read/write/list/shell.
> - `app.go` (≈235-251) — `toolOptions()`: self-dev vira `permit_all` +
>   `EffectiveAutoApprove()` + `SelfManage`.
> - `internal/infrastructure/appconfig/config.go` — `SelfDevConfig`,
>   `EffectiveAutoApprove()` (default true quando self-dev ligado),
>   `EffectiveAllowShell()`.
> - `internal/infrastructure/sandbox/{checker,extract,resolve}_test.go` — cobertura
>   atual (sem casos de path Windows na extração).
> - `docs/plans/permissions-spec.md` — spec original (status desatualizado, ver §1).
> - Relatório da auditoria: este chat (2026-06-11).

## 1. Objective

Fechar os dois gaps que se aplicam ao ambiente real do aw (dev no Windows,
self-dev ligado):

1. **Extração de paths do shell é Unix-only.** `looksLikePath` só reconhece `/`,
   `~/`, `./`, `../`. Caminhos absolutos do Windows (`C:\...`, `\\servidor\share`)
   não são extraídos → em `deny_list`/`permit_all` rodam **sem checagem de path**.
   Os denies built-in (`/etc/shadow`, `/etc/passwd`) também não têm equivalente
   Windows no enforcement do shell.
2. **Self-dev = `permit_all` + auto-approve por padrão.** Com self-dev ligado a
   cerca vira `permit_all` e comandos de shell rodam sem confirmação humana — a
   única defesa do shell vira a heurística + denies built-in (Unix).

Também atualiza o status da `permissions-spec.md` (drift de documentação: a cerca
está largamente implementada, não "not started").

**Fora de escopo:** transformar a heurística de extração num parser de shell
completo (continua heurística, com modelo de ameaça honesto no comentário do
pacote); allowlist de domínios; redesenho dos 4 modos.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **`looksLikePath` reconhece paths Windows também:** letra de drive (`C:\`, `C:/`), UNC (`\\host\share`), e caminhos com separador de barra invertida. A heurística continua heurística (Decision 5), mas passa a **ver** os paths nativos da plataforma onde roda. |
| 2 | **Built-in denied lista por plataforma.** No Windows, além das chaves de home (`~/.ssh`, `~/.gnupg`, `~/.aws` — já expandem certo via `ExpandPath`), incluir os equivalentes sensíveis do Windows que façam sentido (ex.: a pasta de credenciais do usuário, `%APPDATA%` do próprio aw já é coberto pelo `DataDir`/`Protected`). Não inventar lista gigante; portar o essencial e deixar o `DataDir`/`Protected` cobrir o resto. |
| 3 | **A normalização de comparação já é case-insensitive no Windows** (`normalizeForCompare` faz `ToLower`) — manter. A extração precisa alimentar paths nessa normalização; o gap é só na **detecção** (`looksLikePath`), não na comparação. |
| 4 | **Self-dev deixa de auto-aprovar shell por padrão... OU mantém, mas com decisão consciente e documentada.** Avaliar em §5/Phase 3: ou (a) `EffectiveAutoApprove()` passa a default `false` para shell (fs continua auto), ou (b) mantém auto-approve mas adiciona um aviso/limite. A decisão final é registrada na fase, não aqui — o que está travado é que **o comportamento atual não pode ficar implícito**. |
| 5 | **A extração continua heurística e fail-safe no sentido do permit_list:** comando sem path verificável em `permit_list` segue bloqueado se não for safe-no-path. Não relaxar isso ao adicionar suporte a Windows. |
| 6 | **Nenhuma mudança no chokepoint único nem no contrato das ações.** `resolve()`/`CheckCommand()` continuam sendo os pontos de decisão; só ficam mais espertos. |
| 7 | **Toda recusa continua estruturada e honesta** (`blocked` + `reason` verbatim ao modelo); o agente não repete operação bloqueada. |

## 3. What already exists — reuse, don't reinvent

- `ExpandPath`/`normalizeForCompare`/`isInsideFolder` (`checker.go`) — já tratam
  Windows na **comparação** (ToLower, ToSlash). Reusar; o trabalho é na detecção.
- `tokenize` (`extract.go`) — já é quote/escape-aware; mantém. Cuidado: no Windows
  o shell é `powershell -Command`, com regras de aspas/escape próprias — a
  heurística não precisa ser perfeita, mas o comentário do pacote deve registrar
  a limitação por shell.
- `unsafeOperators` + `IsSafeNoPathCommand` — a porta de saída do permit_list para
  comandos sem path. Reavaliar a lista no contexto Windows (ex.: `pwd`/`whoami`
  existem no PowerShell? `whoami` sim; `pwd` é alias de `Get-Location`).
- `DataDir`/`Protected` na `Policy` — já protegem o vault/config em todo modo,
  incl. `permit_all`. Cobrem boa parte do "sensível no Windows" sem lista nova.
- `EffectiveAutoApprove()`/`EffectiveAllowShell()` (`config.go`) — os pontos para
  ajustar a postura do self-dev (Phase 3), já com `*bool` (nil = default).

## 4. Gaps a corrigir (a auditoria)

| # | Gap | Onde | Severidade |
|---|-----|------|-----------|
| G1 | `looksLikePath` não reconhece path absoluto Windows (`C:\`, UNC) → não extraído → não checado em deny_list/permit_all | `extract.go` `looksLikePath` | Média/Alta |
| G2 | Built-in denies só Unix no enforcement do shell | `domain/sandbox.go` + uso no checker | Média |
| G3 | Self-dev = permit_all + auto-approve shell por padrão (sem confirmação humana) | `app.go` + `config.go` | Média |
| G4 | Drift: `permissions-spec.md` diz "not started" mas está implementada | `docs/plans/permissions-spec.md` | Baixa (organização) |
| G5 | Heurística driblável (aspas internas, `$()`, env, flags `--file=`) — modelo de ameaça assumido; reforçar só a documentação/defesa em profundidade | `extract.go` | Baixa (assumido) |

## 5. Phases

### Phase 1 — Extração de path consciente de Windows (G1)

**Status:** implemented (2026-06-11).

1. Estender `looksLikePath` para reconhecer: drive letter (`^[A-Za-z]:[\\/]`),
   UNC (`^\\\\`), e tokens com separador `\` que pareçam path. Manter os prefixos
   Unix atuais. Não classificar URLs (`http(s)://`) como path (já tratado).
2. Ajustar `redirectPattern`/`curlPattern`/`wgetPattern` se fizer sentido para
   alvos com path Windows (avaliar; redirecionamento `>` no PowerShell aceita path
   Windows). Conservador: cobrir o comum, documentar o resto.
3. `resolveAgainstCwd` e o feed para `IsPathAllowed` já normalizam — verificar que
   um path Windows extraído chega corretamente ao `normalizeForCompare`.
4. **Accept:** em `deny_list`/`permit_all`, `type C:\Users\<user>\.ssh\id_rsa`
   (ou `cat` no path Windows) é **detectado** e checado; um path Windows dentro
   da deny list é bloqueado. Testes unitários em happy-path Go (sem depender de
   rodar no Windows — testar a função de detecção/normalização diretamente).

### Phase 2 — Built-in denies por plataforma (G2)

**Status:** implemented (2026-06-11).

1. Tornar `sandboxBuiltinDenied()` sensível à plataforma: no Windows, garantir que
   as entradas de home (`~/.ssh` etc.) cobrem o equivalente real e adicionar o
   essencial (avaliar pasta de credenciais do usuário). Confirmar que `DataDir`/
   `Protected` já cobrem `%APPDATA%\aw` (vault/config).
2. Não duplicar o que `DataDir`/`Protected` já faz; lista mínima e justificada.
3. **Accept:** no Windows, um comando de shell que referencia `~/.ssh` (resolvido)
   é bloqueado em todo modo, incl. `permit_all`; o vault/config do aw segue
   inalcançável em todo modo.

### Phase 3 — Postura do self-dev no shell (G3)

**Status:** implemented (2026-06-11). Decisão escolhida: opção (b). O fluxo
self-dev continua auto-aprovando quando `allowShell`/`autoApprove` estão ativos,
para não quebrar o auto-dev existente, mas o prompt injetado agora registra
explicitamente que shell execution está intencionalmente habilitado e pode ser
auto-aprovado pela configuração runtime. `docs/SELFCODE.md` também documenta a
postura e limita o uso ao escopo da tarefa.

1. Decidir e registrar: o self-dev deve continuar auto-aprovando **shell** por
   padrão? Opções:
   - (a) `EffectiveAutoApprove()` separa fs (auto) de shell (pede confirmação por
     padrão no self-dev), com override explícito em `config.json`.
   - (b) mantém auto-approve mas só com `AllowShell` explícito + aviso no prompt.
2. Implementar a opção escolhida sem quebrar o fluxo de auto-desenvolvimento
   existente (o agente ainda precisa conseguir trabalhar; o objetivo é não rodar
   shell arbitrário totalmente silencioso por default).
3. **Accept:** com self-dev ligado e config default, um comando de shell que toca
   path fora do repo passa por uma barreira explícita (confirmação ou aviso
   registrado), não roda 100% silencioso.

### Phase 4 — Doc drift + modelo de ameaça (G4 + G5)

**Status:** implemented (2026-06-11).

1. Atualizar o **Status** da `permissions-spec.md` para "implemented" com a lista
   real de chokepoints cobertos (read/write/list/shell, fs.*, git.exec,
   shell.exec, browser.navigate, notepad) e a data.
2. Reforçar o comentário do pacote `sandbox` (`extract.go`) sobre a limitação por
   shell (PowerShell vs sh, obfuscação assumida) e que o backstop é a confirmação
   por comando — agora coerente com a decisão da Phase 3.
3. Atualizar `docs/SELFCODE.md` (seção Permissions/sandbox) se o comportamento de
   confirmação do self-dev mudar.
4. **Accept:** spec e SELFCODE refletem o estado real; nenhum doc diz "not started"
   para algo implementado.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend
```

Os testes em `internal/architecture` devem continuar verdes. Adicionar casos de
unidade para a detecção de path Windows (`looksLikePath`/`ExtractPaths`) e para a
lista de denies por plataforma, **testando as funções diretamente** (não depende
de rodar o binário no Windows). Atualizar `docs/SELFCODE.md` e a
`permissions-spec.md` nas fases que mudam comportamento/estado.

## 7. Risks / attention

- **Não relaxar o permit_list ao adicionar Windows** (Decision 5): o objetivo é
  *detectar mais* paths, não criar caminho novo de liberação. Um path Windows
  detectado deve ser checado como qualquer outro.
- **Heurística continua heurística** (G5): reconhecer path Windows reduz o gap
  óbvio, mas não fecha obfuscação (aspas internas, `$()`, env, flags `--file=`).
  Não prometer blindagem total no shell; o backstop honesto é a confirmação.
- **PowerShell tem regras de aspas/escape diferentes do `sh`** — `tokenize` é
  best-effort; documentar e não tentar parser perfeito.
- **Mudar o auto-approve do self-dev altera o fluxo de auto-desenvolvimento** —
  validar que o agente ainda consegue trabalhar; dizer a mudança claramente no
  resumo. Não travar o self-dev a ponto de inutilizá-lo.
- **TOCTOU/symlink já é defendido** em `ResolveAndCheck` (realpath do ancestral) —
  não regredir essa defesa ao mexer na extração.
- **Backend só recarrega após restart do app** — dizer nos resumos finais.
- **Sem `git add -A`** (working copy compartilhada); commits pequenos, um por fase
  onde fizer sentido. Nomes de arquivo com hífen, nunca travessão.
