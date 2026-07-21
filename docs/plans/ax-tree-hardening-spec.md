# Spec — AX Tree Hardening (paridade `ui.*` ↔ `browser.*`)

> **Status:** implemented (2026-06-11, commit 87df9a2). Mantida como registro de
> design. Mesma série e convenções das specs anteriores
> (`workspace-modules-spec.md`, `permissions-spec.md`,
> `sidebar-modules-aw2-port-spec.md`) — mesmos gates. **Não foi** redesign: foi
> endurecer e aproximar as duas árvores de acessibilidade que já existiam. As
> fases abaixo são o registro do plano executado.
>
> Arquivos-fonte (a verdade atual, ler antes de mexer):
>
> - `frontend/src/lib/ui-automation.ts` — AX tree da UI interna do aw (React),
>   alimenta `ui.snapshot/click/fill/screenshot` via round-trip Wails `ui:command`.
> - `frontend/src/lib/ui-automation.test.ts` — 8 testes unitários (happy-dom).
> - `internal/infrastructure/browser/pagescripts.go` — JS injetado via CDP em
>   páginas arbitrárias; alimenta `browser.snapshot/click/fill`.
> - `internal/infrastructure/browser/commands.go` — dispatch dos scripts no CDP.
> - `internal/infrastructure/browser/manager_test.go` — `TestChromeEndToEnd`
>   (integração CDP, **pulado** quando não há Chrome instalado).
> - `internal/infrastructure/tools/aw_browser.go` — registro das ações `browser.*`.
> - `docs/SELFCODE.md` (seções `ui.*` e `browser.*`) — descrição das ações.

## 1. Objective

Fechar os gaps de qualidade e a divergência de paridade entre as duas árvores de
acessibilidade do aw, identificados na auditoria de 2026-06-11. Hoje a árvore da
**UI interna** é rica e testada; a do **Agent Browser** é um subconjunto mais
pobre, com vazamento de glyphs decorativos no nome acessível, sem travessia de
iframe, `isVisible` raso, e **sem teste unitário próprio** (só o teste de
integração CDP, que é pulado sem Chrome). Esta spec não muda o contrato das
ações (`snapshot/click/fill` por ref ou seletor); só melhora a fidelidade da
árvore e a cobertura de testes.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **O contrato das ações não muda.** `ui.*` e `browser.*` continuam com os mesmos parâmetros (`max`, `ref`, `selector`, `value`, `tab`) e o mesmo formato de saída (árvore YAML-like com refs `eN`, `nodes`, e no browser `url`/`title`). Nada de novo comando. |
| 2 | **Mascarar segredos continua inegociável.** Campos `password` sempre ecoam `••••••` em `value` e no retorno do `fill`, nas duas implementações. Nenhuma mudança pode regredir isso. |
| 3 | **Os refs continuam efêmeros e pré-ordem** (`e1, e2, ...`), atribuídos antes dos filhos, resetados a cada snapshot. `click`/`fill` por ref desconhecido/desconectado seguem com erro claro pedindo novo snapshot. |
| 4 | **A versão interna (`ui-automation.ts`) é a referência de qualidade de nome.** O `accessibleText` que remove subárvores decorativas (`aria-hidden`, `role=presentation/none`) é o comportamento correto; o browser deve convergir pra ele, não o contrário. |
| 5 | **Roles específicos do design system do aw** (`data-slot` card/card-title/switch, `aria-labelledby`, field-labels) ficam **só** na versão interna. Páginas web arbitrárias não têm esse design system; não portar isso pro browser. Paridade = qualidade do nome/visibilidade/estrutura, não cópia 1:1. |
| 6 | **Profundidade limitada por `max`** continua sendo o teto de segurança contra árvores gigantes. Travessia de iframe/shadow conta nós no mesmo orçamento `max` (não pode estourar o teto somando frames). |
| 7 | **Sem dependência de Chrome instalado para o gate passar.** A lógica do snapshot do browser precisa de teste unitário que rode em happy-dom (mesma infra do `ui-automation.test.ts`), independente do `TestChromeEndToEnd`. |

## 3. What already exists — reuse, don't reinvent

**Na UI interna (`ui-automation.ts`), reaproveitar como modelo:**
- `accessibleText(el)` — concatena texto pulando `aria-hidden`/`presentation`/`none`.
- `computeRole`/`computeName`/`computeStates`/`computeValue` — lógica madura.
- `isVisible(el)` — base a ser endurecida (ver §5).
- `buildTree`/`serialize`/`countNodes` — pré-ordem + cap por `max`.

**No browser (`pagescripts.go`):**
- `snapshotJS`/`resolveTargetJS`/`clickJS`/`fillJS` — strings JS injetadas via CDP.
- Já cobre `FORM`/`TABLE` (roles extras úteis em páginas reais) — manter.
- `window.__awBrowserRefs` como mapa de refs por página — manter.

**Infra de teste:**
- `frontend/vitest.config.*` (`environment: 'happy-dom'`) — usar pra um novo teste
  unitário da lógica de snapshot do browser, extraindo o JS pra um módulo testável
  ou replicando o algoritmo num helper com paridade verificada.
- `manager_test.go:TestChromeEndToEnd` — manter como teste de integração; adicionar
  asserts pros casos novos (iframe, glyph) quando Chrome estiver presente.

## 4. Gaps a corrigir (a auditoria)

| # | Gap | Onde | Severidade |
|---|-----|------|-----------|
| G1 | Nome usa `el.textContent` cru → glyphs decorativos (ícones Material Symbols, spans `aria-hidden`) vazam pro nome acessível | `pagescripts.go` `name()` | Média |
| G2 | Sem travessia de `<iframe>` — conteúdo dentro de frames não aparece no snapshot | `pagescripts.go` `visit()` | Média |
| G3 | `isVisible` raso: só `hidden`/`display`/`visibility`. Ignora `opacity:0`, off-screen, tamanho zero, `inert` | ambos | Baixa/Média |
| G4 | Sem teste unitário da lógica de snapshot do browser; só integração CDP pulada sem Chrome | `internal/infrastructure/browser` | Média |
| G5 | (opcional) Sem travessia de Shadow DOM. Hoje a UI do aw é React puro, então não morde; documentar como limitação consciente e decidir se entra | ambos | Baixa |

## 5. Phases

### Phase 1 — Browser name parity (G1) — done

1. Portar a semântica do `accessibleText` pro `name()` do `snapshotJS`: ao montar
   o nome de `button/link/heading/listitem/option/tab/menuitem`, percorrer os
   filhos pulando `aria-hidden="true"` e `role` em `presentation`/`none`, em vez
   de usar `el.textContent` cru.
2. Manter a ordem de precedência existente (`aria-label` → labels/placeholder →
   texto acessível → `title` → `alt`).
3. **Accept:** numa página com `<button><span aria-hidden="true">content_copy</span>Copiar</button>`,
   `browser.snapshot` retorna o nome `"Copiar"`, sem o glyph.

### Phase 2 — Iframe traversal no browser (G2) — done

1. No `visit()` do `snapshotJS`, ao encontrar um `IFRAME` acessível (mesma origem),
   descer no `contentDocument.body` e continuar a numeração de refs no mesmo
   orçamento `max`. Frames cross-origin (sem `contentDocument`) viram um nó folha
   marcado (ex.: `iframe "<cross-origin>" [eN]`), sem quebrar.
2. `resolveTargetJS`/`clickJS`/`fillJS` precisam resolver refs que vivem dentro de
   `contentDocument` — guardar o elemento real no `__awBrowserRefs` já resolve
   (o ref aponta pro elemento, não pro seletor), validar `isConnected` no doc certo.
3. **Accept:** página com um iframe same-origin contendo um `<button>` aparece no
   snapshot com ref próprio, e `browser.click` nesse ref funciona. Cross-origin não
   derruba o snapshot.

### Phase 3 — `isVisible` endurecido (G3) — done

1. Estender `isVisible` (nas duas implementações, mantendo paridade) pra também
   rejeitar: `style.opacity === '0'`, elemento com `getClientRects().length === 0`
   (off-screen/zero-size) e ancestral/elemento com atributo `inert`.
2. Cuidado com custo: `getComputedStyle` já é chamado; reusar. Não chamar layout
   caro por nó além do necessário.
3. **Accept:** elementos `opacity:0`, `inert` e de tamanho zero deixam de aparecer
   no snapshot; teste unitário cobre cada caso na versão interna.

### Phase 4 — Teste unitário do snapshot do browser (G4) — done

1. Tornar a lógica do `snapshotJS` testável em happy-dom sem CDP: extrair o
   algoritmo pra um módulo TS compartilhável (ou um helper de teste que injeta o
   mesmo JS via `eval`/`new Function` num DOM happy-dom) e cobrir os casos de
   G1/G2/G3 + máscara de senha + cap por `max`.
2. Garantir que o teste roda no gate padrão (`npm run build:frontend`) sem Chrome.
3. Reforçar `TestChromeEndToEnd` com asserts pros casos novos quando Chrome existir.
4. **Accept:** `cd frontend && npm run test` cobre snapshot do browser; gate verde
   sem Chrome instalado.

### Phase 5 (opcional) — Shadow DOM (G5) — documented limitation

1. Decisão explícita: **não entra agora**. Hoje a UI do aw é React puro e
   páginas web variam; piercing de Shadow DOM aberto fica fora deste hardening.
2. Documentado em `docs/SELFCODE.md` como limitação conhecida das ações
   `ui.*`/`browser.*` (Shadow DOM aberto não é percorrido).

## 5.1 Implementation notes

- Browser snapshot: `internal/infrastructure/browser/pagescripts.go` agora usa
  `accessibleText` equivalente ao da UI interna, ignora subárvores decorativas
  (`aria-hidden`, `role=presentation|none`), atravessa iframes same-origin e
  transforma iframes inacessíveis em folha `iframe "<cross-origin>" [eN]`.
- UI snapshot: `frontend/src/lib/ui-automation.ts` filtra `opacity:0`, `inert`
  e elementos explicitamente sem caixa/off-screen.
- Teste sem Chrome: `frontend/src/lib/browser-page-snapshot.test.ts` cobre G1,
  G2, G3, máscara de senha e cap por `max` em `happy-dom`.
- Integração CDP: `internal/infrastructure/browser/manager_test.go` ganhou
  asserts para glyph decorativo, máscara de senha e botão dentro de iframe, mas
  continua pulando quando não há Chrome instalado.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend   # lint + typecheck + vitest + vite build
```

Os testes em `internal/architecture` devem continuar verdes. Atualizar
`docs/SELFCODE.md` (seções `ui.*` e `browser.*`) em toda fase que mudar o
comportamento observável da árvore.

## 7. Risks / attention

- **Não regredir a máscara de senha** (Decision 2). Qualquer mudança em `name`/
  `value`/iframe precisa manter `password` → `••••••` nas duas implementações.
- **Iframe cross-origin lança exceção** ao acessar `contentDocument`. Fail closed:
  capturar e emitir nó folha marcado, nunca deixar o snapshot inteiro estourar.
- **`getClientRects`/layout em `isVisible`** pode custar caro em páginas enormes;
  medir contra o cap `max` e parar cedo. Não transformar snapshot O(n) em O(n²).
- **Paridade não é cópia 1:1** (Decision 5): roles do design system do aw ficam só
  na versão interna; o que converge é qualidade de nome, visibilidade e estrutura.
- **Conteúdo de página web é entrada não-confiável** (prompt-injection). Não logar
  texto de página nem valores de `fill`; manter o aviso ao modelo no prompt do
  módulo browser.
- **Backend só recarrega após restart do app** — dizer isso nos resumos finais.
  Mudanças no `frontend/src/lib/ui-automation.ts` precisam de rebuild do renderer.
- **Sem `git add -A`** (working copy compartilhada); commits pequenos, um por fase
  onde fizer sentido. Nomes de arquivo com hífen, nunca travessão.
```
