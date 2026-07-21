# Spec — Vault Memory Hygiene (port-back do SecureString/secureMemory do AW2)

> **Status:** implemented (2026-06-11, phases 1-3: commits 3d6ffdf, 42e4c8a,
> 75a0c8c; Phase 4 opcional não executada). Mantida como registro de design.
> Quarta spec da série de hardening (depois de `ax-tree-hardening-spec.md`,
> `agent-browser-hardening-spec.md`, `permissions-sandbox-hardening-spec.md`).
> Mesmas convenções e mesmos gates. As fases abaixo são o registro do plano
> executado (Phase 4 ficou de fora por ser opcional).
>
> **Origem (auditoria 2026-06-11):** comparando AW2 × aw, a criptografia *em
> repouso* do aw estava correta (VFS adiantum cifra o arquivo inteiro; salt de 128
> bits; recovery key auto-criada que permite trocar a senha). O que o aw havia
> **regredido** em relação ao AW2 era a **higiene de memória das chaves/segredos**:
> o AW2 mantinha a senha e a master key **seladas (cifradas) na RAM** e **zerava os
> buffers** após uso; o aw guardava a chave derivada como uma `string` Go em texto
> puro pela sessão inteira e nunca a apagava, e devolvia segredos como string crua
> (corrigido nos commits acima).
> Esta spec **porta de volta** o comportamento do AW2 — não inventa nada novo.
>
> Arquivos-fonte AW2 (a referência a portar):
>
> - `~/AgentWorkspace2/src/shared/infrastructure/secure-memory.ts` — classe
>   `SecureMemory`: ESK (Ephemeral Session Key) gerada uma vez por processo;
>   `seal(plaintext) → SealedValue` (AES-GCM com a ESK), `unseal(sealed)`,
>   `destroy()` (zera a ESK), zera buffers intermediários (`fill(0)`).
> - `~/AgentWorkspace2/src/shared/infrastructure/secure-string.ts` — `SecureString`:
>   guarda só a forma selada; `use(fn)` roda fn com o plaintext transitório,
>   `unwrap()`, `destroy()` (zera os buffers).
> - `~/AgentWorkspace2/src/shared/infrastructure/sqlite-vault.ts` — uso real:
>   `_lastPassword: SecureString` (linha 72; nunca texto puro), `new SecureString(password)`
>   no create/unlock (130/188), `_lastPassword.use(pw => deriveKey(...))` (241),
>   `masterKeyBuf.fill(0)` após abrir o db (314/320), `getSecretSecure()` devolve
>   segredo embrulhado em `SecureString` (377-383).
>
> Arquivos-fonte aw (o estado atual a endurecer):
>
> - `internal/infrastructure/vault/vault.go`:
>   - `keyHex string` (≈45) — chave derivada, em texto puro na RAM; `lockLocked`
>     (≈1755) faz `v.keyHex = ""` (não zera os bytes).
>   - `deriveKey` (≈1829): `scrypt.Key(...)` → `hex.EncodeToString` (string).
>   - `Create`/`Unlock`/`ChangePassword`/`RecoverWithKey` — pontos onde a senha e a
>     chave transitam como string.
>   - `GetSecret`/`SetSecret` (≈476-512) — segredos como string crua.
>   - `decryptRecoveryMasterKeyLocked` (≈1424) — master key desembrulhada em string.
> - `internal/infrastructure/vault/vault_test.go` — cobertura atual.
> - Relatório da auditoria + comparação AW2: este chat (2026-06-11).

## 1. Objective

Restaurar no aw a higiene de memória de chaves e segredos que o AW2 já tinha e
que se perdeu no port para Go:

1. **A senha e a master key não devem ficar em texto puro na RAM** pela sessão
   inteira. No AW2 ficavam seladas (cifradas com a ESK) via `SecureString`; no aw
   ficam como `string` Go imutável, nunca apagada.
2. **Buffers de chave devem ser zerados após uso** (`fill(0)` no AW2). No aw nada
   é zerado — `keyHex = ""` só troca a referência; os bytes ficam no heap até o GC
   (e nem assim são apagados).
3. **Segredos lidos do vault devem poder ser embrulhados** (equivalente ao
   `getSecretSecure` do AW2) para não vazarem em texto puro mais do que o necessário.

**Fora de escopo:** trocar o esquema em repouso (adiantum + AES-GCM continuam);
trocar o fluxo de recovery (funciona); mudar o salt (já é 128 bits aleatório).
**scrypt N=2^14** e a codificação da recovery key **NÃO são alvos primários** —
são paridade com o AW2 (battle-tested), tratados como melhoria opcional na Phase 4.

## 2. Locked decisions (do not reopen)

| # | Decision |
|---|----------|
| 1 | **É um port-back do AW2, não um redesenho.** Replicar a semântica de `SecureMemory` (ESK por processo + seal/unseal/destroy) e `SecureString` (use/unwrap/destroy) em Go. Em dúvida sobre comportamento, abrir o arquivo `.ts` do AW2 e copiar. |
| 2 | **Chaves/segredos sensíveis em memória passam a ser `[]byte` zerável**, não `string` Go (strings são imutáveis e não dá pra apagar). A `keyHex` exposta ao `PRAGMA hexkey` tem vida curta e o buffer é zerado logo após o `openEncrypted`. |
| 3 | **A senha de unlock é selada imediatamente** (equivalente ao `_lastPassword = new SecureString(password)`), nunca mantida como string viva. `ChangePassword`/`RecoverWithKey` re-selam. |
| 4 | **`Lock()` zera os buffers**, não só troca referência: zerar o buffer da master key e destruir o `SecureString` da senha. O ticket de auto-lock (30s, default 15 min) continua chamando o mesmo caminho — ganha a zeroização de graça. |
| 5 | **Honestidade sobre limites do Go:** GC pode copiar/mover heap, não há pinning de string, swap/core dump existem. O port reduz a janela e a superfície (sem texto puro persistente), mas **não promete blindagem total**. Documentar isso no comentário do pacote, como o AW2 documentava. `mlock`/`memguard` é bônus avaliável, não requisito. |
| 6 | **A ESK (Ephemeral Session Key) é gerada uma vez por processo** com `crypto/rand`, vive só em `[]byte`, e é destruída no shutdown/lock global. Mesma ideia do `SecureMemory` do AW2. |
| 7 | **Nada de chave/segredo em log, jamais** (já é o caso) — manter e cobrir com teste nos caminhos críticos. |
| 8 | **Compatibilidade:** isto é mudança só de *como a chave vive na memória*. NÃO muda formato em disco, NÃO muda derivação, NÃO invalida vaults nem recovery keys existentes. Unlock/recovery/change-password seguem idênticos do ponto de vista do usuário. |

## 3. What already exists — reuse, don't reinvent

- **AW2 já tem o design completo e testado** (`secure-memory.test.ts`,
  `secure-string` em uso no `sqlite-vault.ts`). O trabalho é **traduzir para Go**,
  não projetar do zero.
- `crypto/cipher` + `crypto/aes` (já importados no `vault.go`) — usar a mesma
  primitiva (AES-GCM) para a ESK selar/desselar, como o AW2.
- `crypto/rand` (`randomBytes` já existe no `vault.go`) — para a ESK e IVs.
- Estrutura de lock do Vault (`v.mu`, `lockLocked`, `reopenLocked`) — os pontos de
  zeroização entram aí; não mexer no contrato.
- `vault_test.go` — estender com testes de selagem/zeroização.

## 4. Gaps a corrigir (a auditoria)

| # | Gap | Onde | Severidade |
|---|-----|------|-----------|
| **G1** | **Master key/senha em texto puro na RAM a sessão inteira** (regressão do AW2) | `keyHex string`, `Create`/`Unlock` | **Média (regressão real)** |
| **G2** | **Buffers de chave nunca zerados** — `keyHex = ""` não apaga bytes | `lockLocked`, `ChangePassword`, `RecoverWithKey` | **Média (regressão real)** |
| G3 | Segredos devolvidos como string crua (sem equivalente a `getSecretSecure`) | `GetSecret` | Baixa/Média |
| G4 | scrypt `N=2^14` (paridade com AW2, dá pra modernizar) | `deriveKey` | Baixa (opcional) |
| G5 | `generateRecoveryKey` com viés de módulo (herdado do AW2) | `generateRecoveryKey` | Baixa (opcional) |

## 5. Phases

### Phase 1 — Portar `secureMemory` + `SecureString` para Go (G1 base)

1. Criar um pacote Go (ex.: `internal/infrastructure/securemem`) com:
   - `SecureMemory`: ESK em `[]byte` (32 bytes, `crypto/rand`); `Seal([]byte) SealedValue`
     (AES-GCM), `Unseal(SealedValue) []byte`, `Destroy()` (zera a ESK); zera buffers
     intermediários.
   - `SecureString`/`SecureBytes`: guarda só a forma selada; `Use(func([]byte))`
     com plaintext transitório zerado ao fim, `Destroy()`.
2. Portar os testes do AW2 (`secure-memory.test.ts`) para Go: seal→unseal round-trip,
   destroy zera, unseal após destroy falha, buffers intermediários zerados.
3. **Accept:** pacote com testes verdes; comportamento equivalente ao AW2;
   `golangci-lint` limpo.

### Phase 2 — Usar no Vault: senha selada + master key zerada (G1 + G2)

1. Trocar `keyHex string` por uma representação zerável (`[]byte` selado ou
   `SecureBytes`). A string hex exposta ao `PRAGMA hexkey` é montada na hora,
   usada, e o buffer zerado logo após `openEncrypted`.
2. Selar a senha de unlock imediatamente (equivalente a `_lastPassword`); usar via
   `Use(...)` só quando precisar re-derivar; destruir no lock e re-selar em
   change-password/recover.
3. `lockLocked` (e os pontos de troca em `ChangePassword`/`RecoverWithKey`) passam a
   **zerar** os buffers, não só atribuir `""`/`nil`. `decryptRecoveryMasterKeyLocked`
   zera o buffer da master key desembrulhada após uso (como `masterKeyBuf.fill(0)`).
4. **Accept:** após `Lock()` (manual e via auto-lock), não há `[]byte` de master
   key/senha vivo e não-zerado referenciado pelo Vault; teste verifica zeroização
   onde o Go permite inspecionar; unlock→lock→unlock funciona; recovery e
   change-password intactos.

### Phase 3 — Segredos embrulhados (G3)

1. Adicionar caminho equivalente ao `getSecretSecure` do AW2: ler segredo e
   devolver embrulhado (`SecureBytes`/`SecureString`) para os consumidores que
   conseguem usar `Use(...)`; manter `GetSecret` string para compatibilidade onde
   o embrulho não couber, mas minimizar a vida do plaintext.
2. **Accept:** segredos sensíveis podem ser consumidos sem materializar string
   persistente; teste de round-trip + zeroização do plaintext transitório.

### Phase 4 — Melhorias opcionais + testes/docs (G4 + G5)

1. **(Opcional)** Modernizar o KDF da *senha* (Argon2id ou scrypt N maior),
   benchmarkado (unlock < ~1s no M1/Avell). Como estamos em dev sem dados de
   produção, pode trocar sem migração (vault antigo se recria). Decidir ao
   implementar; **não bloqueia** as fases 1-3.
2. **(Opcional)** Corrigir o viés de módulo de `generateRecoveryKey` (rejection
   sampling), preservando o formato `XXXX-XXXX-...` e ≥128 bits.
3. Teste que falha se chave/segredo aparecer em log nos caminhos
   create/unlock/recover/change-password (Decision 7).
4. Atualizar `docs/SELFCODE.md` (seção vault/cripto) com a política de higiene de
   memória portada do AW2 e os limites do Go (Decision 5).
5. **Accept:** docs refletem o novo comportamento; suíte verde; gate completo verde.

## 6. Gates (run at the end of EACH phase)

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate            # lint + go test + wails build
cd frontend && npm run build:frontend
```

Os testes do novo pacote `securemem` e os de zeroização do vault são obrigatórios.
Benchmark de KDF (se a Phase 4 mexer nisso) é informativo, não gate.

## 7. Risks / attention

- **Zeroização em Go é best-effort** (Decision 5) — strings imutáveis, GC que copia
  heap, swap, core dump. O port reduz a janela e elimina o texto puro persistente,
  mas não blinda contra um atacante com acesso total à RAM. Não prometer mais que
  isso; documentar honestamente como o AW2 fazia.
- **Não quebrar unlock/recovery/change-password** (Decision 8) — é mudança de
  memória, não de formato. Testar os três fluxos de ponta a ponta antes de
  declarar pronto. O risco é introduzir bug de uso-após-zerar (zerar um buffer
  ainda em uso) — cobrir com teste.
- **Concorrência:** a ESK e os buffers selados são acessados sob `v.mu`; garantir
  que `Use(...)` não vaza o `[]byte` transitório para fora do escopo (não guardar
  referência, zerar ao fim).
- **scrypt/recovery encoding são opcionais (Phase 4)** — não são regressão, são
  paridade com o AW2. Não deixar a discussão de KDF atrasar o ganho real (fases 1-3).
- **Backend só recarrega após restart do app** — dizer nos resumos finais.
- **Sem `git add -A`** (working copy compartilhada); commits pequenos, um por fase.
  Nomes de arquivo com hífen, nunca travessão.
