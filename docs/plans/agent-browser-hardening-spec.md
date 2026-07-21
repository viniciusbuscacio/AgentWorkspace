# Spec — Agent Browser Hardening (postura de segurança)

> **Status:** implemented (2026-06-11, commit 0eac74a). Mantida como registro de
> design. Mesma série e convenções das specs anteriores
> (`workspace-modules-spec.md`, `permissions-spec.md`,
> `sidebar-modules-aw2-port-spec.md`, `ax-tree-hardening-spec.md`) — mesmos gates.
> Fechou os riscos da auditoria de segurança do Agent Browser (2026-06-11).
> **Não** redesenhou o CDP core nem o contrato das ações `browser.*`; mudou a
> *postura de segurança* (perfil padrão, alcance de navegação, furo na cerca de
> `file://`). As fases abaixo são o registro do plano executado.
>
> Arquivos-fonte (a verdade atual, ler antes de mexer):
>
> - `internal/infrastructure/browser/manager.go` — launch, perfis, `denyDownloads`,
>   `gracefulQuit`, `processRunning`, `launchArgs`.
> - `internal/infrastructure/browser/commands.go` — `runPageCommand`
>   (navigate/snapshot/click/fill/screenshot), `pickPage`, `navigate`.
> - `internal/infrastructure/browser/cdp.go` — sessão CDP (Runtime.evaluate etc.).
> - `internal/infrastructure/browser/pagescripts.go` — JS de snapshot/click/fill.
> - `internal/infrastructure/tools/aw_browser.go` — registro das ações `browser.*`,
>   `fileURLPath` + `sandbox.ResolveAndCheck` (só em `browser.navigate` hoje).
> - `internal/domain/browser.go` — `BrowserStartOptions`, `ParseBrowserProfile`,
>   `PersonalProfile()` (perfil vazio == personal).
> - `internal/infrastructure/sandbox` — o checker de permissões (cerca de disco).
> - `internal/infrastructure/browser/manager_test.go` — `TestChromeEndToEnd`.
> - `docs/SELFCODE.md` (seção Agent Browser) — descrição das ações ao modelo.
> - Relatório da auditoria: este chat (2026-06-11).

## 1. Objective

Endurecer a postura de segurança do Agent Browser, fechando a combinação de
riscos da auditoria: (a) o perfil **padrão** é o navegador pessoal logado do
usuário, (b) a cerca de `file://` só cobre `browser.navigate` e pode ser furada
por `browser.click` num link, e (c) não há nenhum limite de para onde o agente
pode navegar (localhost, IPs internos, metadados de nuvem). O CDP core e o
contrato das ações continuam iguais; o que muda é o default seguro, o alcance
de navegação e a cobertura da cerca.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **Perfil padrão passa a ser o isolado (`aw`), não o pessoal.** Perfil vazio == `aw`. O perfil `personal` continua existindo, mas só com pedido **explícito** (`profile: personal`). Inverte `PersonalProfile()` / `ParseBrowserProfile` em `internal/domain/browser.go`. |
| 2 | **Conteúdo de página web é entrada não-confiável (prompt-injection).** O prompt do módulo já avisa; manter e reforçar. Nenhuma decisão pode tratar texto de página como instrução. |
| 3 | **A cerca de `file://` vale para QUALQUER forma de navegação**, não só `browser.navigate`: clique em link, `fill`+submit, JS-driven. O checker do sandbox é a fonte única; ninguém navega pra `file://` sem passar por ele (fecha o "Risk 7" da permissions-spec de verdade). |
| 4 | **Bloqueio de destinos internos/perigosos por padrão**, independente do perfil: `file://` negado pela cerca, `169.254.169.254` (metadados de nuvem) e demais IPs link-local, loopback (`127.0.0.0/8`, `localhost`, `::1`), e ranges privados (`10/8`, `172.16/12`, `192.168/16`, `100.64/10` CGNAT/Tailscale). **Exceção declarada:** o próprio CDP do aw (já é localhost, mas isso é tráfego interno do app, não navegação do agente). |
| 5 | **Downloads continuam negados (fail closed) nos perfis isolados.** No perfil `personal` (agora opt-in explícito), downloads seguem habilitados — é o navegador do usuário; **mas** o uso de `personal` passa a exigir o aviso do contrato (§5) deixando claro que a cerca de disco não cobre downloads ali. |
| 6 | **Bloqueio = resultado estruturado, fail closed.** Toda recusa retorna `{ "blocked": true, "reason": ..., ... }` como o `browser.navigate` já faz hoje; URL não-parseável é bloqueada (fail closed), nunca liberada por dúvida. O agente não deve repetir operação bloqueada — relata o motivo. |
| 7 | **Sem allowlist configurável nesta spec.** O escopo aqui é *blocklist* de destinos perigosos + default seguro. Uma allowlist de domínios por usuário (em Settings) fica para uma spec futura, se necessário. Não inventar config nova agora. |
| 8 | **O agente vê e testa a cerca, nunca a move** (mesma regra da permissions-spec). Mudar postura de browser, se virar config, é em Settings atrás de diálogo nativo — não via ação `browser.*`. |

## 3. What already exists — reuse, don't reinvent

**No aw:**
- `fileURLPath(rawURL)` (`aw_browser.go`) — já normaliza e detecta `file://`/
  `view-source:file://` com defesa contra tab/CR/LF e host fake. **Reusar** como
  base do checker de navegação; generalizar para também classificar o host
  (interno vs externo), não só `file://`.
- `sandbox.ResolveAndCheck(path, policy)` — o checker de disco. Continua sendo
  quem decide se um `file://` é alcançável.
- `denyDownloads(ctx, port)` via `Browser.setDownloadBehavior` — já fail closed
  nos perfis isolados. Sem mudança de mecanismo, só de quando se aplica (§Decision 5).
- `BrowserStartOptions.PersonalProfile()` + `ParseBrowserProfile()` — o ponto
  único para inverter o default (Decision 1).
- O snapshot/click/fill (`pagescripts.go`/`commands.go`) — sem mudança de
  contrato; o gate de navegação fica **antes** da ação, no Go.

## 4. Riscos a corrigir (a auditoria)

| # | Risco | Onde | Severidade |
|---|-------|------|-----------|
| R1 | Perfil padrão é o navegador pessoal logado | `domain/browser.go` `PersonalProfile()` | Alta |
| R2 | Cerca de `file://` furável por clique em link / JS nav | `aw_browser.go` (só `navigate` checa) | Média/Alta |
| R3 | Sem bloqueio de destinos internos (loopback, privados, `169.254.169.254`) | navegação em geral | Média |
| R4 | Downloads liberados no perfil pessoal (agora opt-in) | `manager.go` `Start`/`denyDownloads` | Média |
| R5 | CDP sem autenticação (porta de debug local) | `launchArgs` `--remote-debugging-port` | Baixa/Média |
| R6 | `restart` mata sessão inteira; `processRunning` sempre false no Windows | `manager.go` `gracefulQuit`/`processRunning` | Baixa |

## 5. Phases

### Phase 1 — Default seguro: perfil isolado (R1) — done

1. Inverter o default em `internal/domain/browser.go`: perfil vazio resolve para
   `aw` (isolado). `PersonalProfile()` só true com `profile: personal` explícito.
   Ajustar `ParseBrowserProfile` e comentários.
2. Atualizar a descrição da ação `browser.start` em `internal/domain/module.go` e
   `docs/SELFCODE.md`: deixar claro que o default é isolado e que `personal` é
   opt-in com aviso (a cerca de disco não cobre downloads no pessoal).
3. **Accept:** `browser.start` sem `profile` sobe o perfil isolado (downloads
   negados via CDP). `browser.start {profile: personal}` usa o navegador do
   usuário e exige o fluxo de aviso/`restart` que já existe.

### Phase 2 — Cerca de navegação única (R2 + R3) — done

1. Criar um checker de navegação em Go (`navigationGuard`), reusando `fileURLPath`
   e adicionando classificação de host: bloqueia `file://` negado pela cerca de
   disco e os destinos internos da Decision 4 (loopback, link-local incl.
   `169.254.169.254`, ranges privados/CGNAT). Resolve hostnames de forma
   conservadora (se resolver para IP interno, bloqueia; fail closed em erro de
   parse).
2. Rotear **toda** navegação por ele:
   - `browser.navigate` — já passa; trocar pelo checker novo.
   - `browser.click` — **antes** de clicar, se o alvo for um `<a href>` (ou
     elemento que dispara navegação), resolver o destino e checar; bloqueado →
     resultado `{ "blocked": true, ... }` sem clicar.
   - Pós-navegação defensiva: depois de `click`/`fill`, conferir a URL corrente
     da página; se virou `file://`/destino interno por navegação indireta (JS,
     meta-refresh), reportar e não fazer snapshot/screenshot do conteúdo
     bloqueado. (Defesa em profundidade, já que clique no JS é difícil de pré-checar.)
3. **Accept:** `browser.click` num link `file:///etc/passwd` retorna `blocked`
   sem navegar; `browser.navigate http://169.254.169.254/...` retorna `blocked`;
   `browser.navigate http://localhost:9300` (MCP interno) retorna `blocked`;
   navegação externa normal segue funcionando.

### Phase 3 — Downloads no pessoal: aviso explícito (R4) — done

1. Manter downloads habilitados só no `personal` (Decision 5), mas o resultado de
   `browser.start {profile: personal}` carrega um `notice` deixando explícito que
   downloads nesse modo **não** passam pela cerca de permissões.
2. (Opcional, avaliar custo) tentar `Browser.setDownloadBehavior {behavior:
   "deny"}` também no pessoal **somente** se não quebrar o uso real do usuário —
   se for arriscado, ficar só no aviso e documentar.
3. **Accept:** iniciar no perfil pessoal devolve o aviso; o comportamento de
   download do usuário não é quebrado silenciosamente.

### Phase 4 — Endurecer restart/Windows (R6) — done

1. `processRunning` no Windows: implementar detecção real (`tasklist`/equivalente)
   em vez de retornar sempre false, para o fluxo de `RequiresRestart` funcionar lá.
2. `gracefulQuit`: avaliar fechar só as janelas controladas quando possível; se
   não for viável, reforçar o `notice` de que a sessão inteira será reiniciada
   (abas restauradas, mas formulário não salvo se perde).
3. **Accept:** no Windows, iniciar com o navegador pessoal aberto sem porta de
   debug reporta `RequiresRestart` (não tenta subir por cima); o aviso de
   perda de dados de formulário aparece antes do restart.

### Phase 5 (opcional) — CDP local mais fechado (R5) — documented limitation

1. Decisão explícita: sem mudança de flags nesta spec. O Chrome/Edge é iniciado
   com porta CDP local dedicada e aw fala com `127.0.0.1`; endurecimento
   adicional de `--remote-allow-origins`/bind fica para uma spec própria se
   houver regressão ou evidência de ganho sem quebrar CDP.
2. Documentado em `docs/SELFCODE.md` como limitação conhecida: enquanto o
   navegador roda, a porta de debug local não tem autenticação própria.

## 5.1 Implementation notes

- `domain.ParseBrowserProfile("")` agora resolve para `aw`; `personal` só com
  `profile: personal` explícito.
- `aw_browser.go` usa um `navigationGuard` único para `browser.navigate`,
  pré-checagem de links em `browser.click` e pós-checagem de `click`/`fill`.
- `navigationGuard` bloqueia `file://` fora da cerca de permissões, URLs sem
  parse/scheme, `localhost`, loopback, link-local, privados e `100.64/10`.
  Hostnames são resolvidos de forma conservadora; erro de DNS bloqueia.
- `browser.Command` ganhou comandos internos não expostos no contrato
  `browser.*`: `navigationTarget` e `currentURL`.
- Perfil pessoal retorna notice explícito sobre downloads fora da cerca de
  permissões; o aviso de restart menciona perda de dados de formulário.
  `processRunning` no Windows usa `tasklist`.
  Testes do guard rodam sem Chrome instalado.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend
```

Os testes em `internal/architecture` devem continuar verdes. Adicionar testes
de unidade para o `navigationGuard` (classificação de host: loopback, link-local
`169.254.169.254`, privados, CGNAT/Tailscale, externo, `file://` permitido/negado,
URL não-parseável → blocked) **sem depender de Chrome instalado**. Reforçar
`TestChromeEndToEnd` com os casos de bloqueio quando houver Chrome. Atualizar
`docs/SELFCODE.md` (seção Agent Browser) em toda fase que mudar comportamento.

## 7. Risks / attention

- **Não quebrar o uso legítimo do navegador.** A blocklist de destinos internos
  pode pegar casos válidos (ex.: usuário quer testar um app local). Por isso a
  allowlist configurável fica para spec futura (Decision 7); aqui o bloqueio é
  default seguro, com motivo claro no resultado pro usuário decidir.
- **Clique que dispara navegação por JS é difícil de pré-checar** — daí a defesa
  em profundidade pós-navegação (Phase 2 step 2.3). Não prometer bloqueio 100%
  antes do clique; o gate real é não entregar o conteúdo de um destino bloqueado.
- **Resolução de hostname → IP** pode ter TOCTOU (DNS rebinding): o IP no check
  pode diferir do IP que o browser usa. Mitigar com a checagem pós-navegação da
  URL/host corrente; documentar a limitação.
- **Inverter o default de perfil muda comportamento observável** de quem já usa o
  browser — dizer isso claramente no resumo e no prompt do módulo. Sessões/
  fluxos que assumiam "navegador pessoal por padrão" agora pedem `profile: personal`.
- **Fail closed sempre**: URL não-parseável, erro de DNS, `contentDocument`
  inacessível → bloquear/relatar, nunca liberar por dúvida. Manter a máscara de
  senha do `fill`.
- **Backend só recarrega após restart do app** — dizer nos resumos finais.
- **Sem `git add -A`** (working copy compartilhada); commits pequenos, um por fase
  onde fizer sentido. Nomes de arquivo com hífen, nunca travessão.
