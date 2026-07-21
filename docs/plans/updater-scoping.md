# Updater do aw — decisões de scoping

Status: decisões fechadas com o autor em 21/jul/2026 (conversa, uma pergunta
por vez). **Ainda não implementado — este doc é o registro do que foi
decidido.** O design técnico final vira um `docs/updater-design.md` em
inglês, no padrão do go-notepad (`~/go-notepad/docs/updater-design.md`),
quando a implementação começar.

## Contexto

- Referência: updater da família go-apps — biblioteca pública
  [go-updates](https://github.com/viniciusbuscacio/go-updates) (check na
  release do GitHub, semver, download, SHA-256 obrigatório via
  `checksums.txt`, swap com rollback), UX validada no go-notepad
  (v0.2.0 → v0.2.1 rodou o ciclo real).
- aw hoje: repo **privado**, sem tags/releases/CI; versão é texto fixo no
  frontend (`3.0.0-dev`); builds saem do buildgate local assinados com o
  certificado Apple Development do autor; Touch ID/quick unlock dependem do
  Keychain, que confia nessa identidade de assinatura.
- Plano do autor: repo continua privado por ora e **vai abrir ao público
  depois**. O design abaixo serve às duas fases sem retrabalho.

## Decisões

| # | Tópico | Decisão |
|---|--------|---------|
| 1 | Canal (fase privada) | Releases no próprio repo privado; **token fine-grained do GitHub (contents:read) guardado no vault** e configurado em Settings. Só quem tem o token atualiza. |
| 2 | Canal (fase pública) | O token vira **opcional**: sem token, caminho anônimo igual go-notepad. Mesmo código, abrir o repo é só parar de precisar do token. |
| 3 | Plataformas | **macOS primeiro** (arm64, ciclo validado na máquina do autor); Windows numa segunda leva, quando ele puder testar lá. |
| 4 | Swap no macOS | **Bundle `.app` inteiro**: baixa o zip do .app já assinado, rename dance no diretório, relança. Preserva assinatura ⇒ Touch ID/Keychain/TCC continuam valendo. Nunca trocar só o binário (quebraria o selo). |
| 5 | Quem gera a release | **Script local** (`scripts/release.sh`): buildgate → zip do .app → `checksums.txt` → `gh release create`. Sem certificado em CI. |
| 6 | Versionamento | Tags semver a partir de `v3.0.0`; versão injetada no Go via ldflags no buildgate; About e API passam a mostrar a versão real. Build sem tag = `dev` (compara como 0.0.0, igual go-notepad). |
| 7 | Auto-check | **Desligado por padrão** (regra da família: rede só com opt-in). Ligado: 1×/dia, **só com o vault destravado** (o token mora no vault). |
| 8 | Preferências | Por vault (junto do resto), já que o token e o gatilho vivem pós-unlock. |
| 9 | UX | Igual go-notepad: badge discreto no item Settings da sidebar + card **Updates** em Settings; Install and restart / Skip this version (por tag) / Later (7 dias). |
| 10 | Notas da release | Renderizadas como **markdown** (o aw já renderiza markdown no chat/UI). |
| 11 | Restart | Caminho normal de quit (sessões salvam, vault tranca — restore com nota de continuidade já existe). Se houver servidor REST/MCP/Web ouvindo, o card avisa que o restart derruba os clientes conectados. |
| 12 | API do agent | **`update.check` apenas** (risco external, atrás do firewall). Instalar/reiniciar é ação humana, só pela UI. |
| 13 | Biblioteca | Usar go-updates para check/download/checksum, com uma evolução pequena (campo `Token` opcional p/ repo privado — útil pra família toda). O swap de bundle é implementado no aw (app-local), sem generalizar na lib por ora. |
| 14 | DEV instance | Updater **desligado à força** no build-dev (a instância DEV nunca se auto-atualiza). |
| 15 | Aceitação | Igual go-notepad: publicar a primeira versão com updater, depois uma release mínima e rodar o ciclo completo de verdade na máquina do autor. |

## Fica para a fase pública (não bloqueia nada agora)

- **Assinatura para terceiros**: Apple Development só vale nas máquinas do
  autor; público exige Developer ID + notarização (conta paga) — senão
  Gatekeeper bloqueia. Windows: assinatura/SmartScreen.
- **CI de release** (hoje local de propósito): reavaliar quando houver
  notarização.
- Windows: asset, Credential Manager pós-swap, rename dance do exe (o
  go-updates já faz).

## Ordem de implementação (quando começar)

1. **Fase 0 — versão real**: tags, ldflags, About, `scripts/release.sh`.
   Útil por si só, pré-requisito de tudo.
2. **Fase 1 — updater**: token no vault + go-updates com `Token` + swap de
   bundle + card Updates + badge + action `update.check`.
3. **Fase 2 — repo público**: token opcional já funciona; tratar
   assinatura/notarização e (talvez) CI.
