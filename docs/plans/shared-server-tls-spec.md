# Spec — TLS compartilhado para os servidores (Web Access, MCP, REST)

> **Status:** proposta para implementação (2026-06-29)
>
> Supersede `web-server-tls-spec.md` (que assumia Web Access HTTPS-only).
> Aqui **nenhum** servidor é HTTPS-obrigatório: TLS é opcional e default-off nos
> três, usando um certificado **compartilhado** gerenciado num único lugar.

## 1. Objetivo

Hoje os três servidores HTTP do app servem **plaintext**:

- Web Access (`internal/infrastructure/webserver`, porta 9302, bind Tailscale) —
  `httpServer.Serve(listener)`.
- MCP (`internal/infrastructure/mcpserver`, `127.0.0.1:9300`).
- REST (`internal/infrastructure/restserver`, `127.0.0.1:9301`).

Criar um **subsistema TLS compartilhado** (`servertls`) com uma página
**TLS manager** em Settings/Security. O usuário cria/gerencia **um** certificado
(self-signed gerenciado pelo app, ou upload de um próprio) e cada servidor pode,
de forma independente, **ligar ou desligar** HTTPS usando esse certificado. Um
certificado valida hostnames/IPs (não portas), então o mesmo bundle atende às
três portas.

TLS é defesa em profundidade — não substitui as proteções existentes (peer
filter/CIDR + login com senha do vault + rate limit + invalidação de sessão no
Web; loopback + bearer token no MCP/REST). O maior ganho real é no Web Access
(cross-host na tailnet); em MCP/REST (loopback) é marginal mas suportado para
clientes que exijam `https`.

## 2. Decisões travadas (não reabrir)

| # | Decisão |
|---|---------|
| 1 | **Nenhum servidor tem TLS obrigatório.** Cada um (Web/MCP/REST) tem um toggle `tlsEnabled` **default false**. Off = plaintext (comportamento atual, inalterado). On = HTTPS com o cert compartilhado. |
| 2 | **Criação de certificado é 100% manual** no TLS manager. Não auto-gerar na instalação, nem lazy ao ligar o toggle. Ações: **Create self-signed**, **Regenerate**, **Upload custom**. |
| 3 | **Ligar TLS sem certificado falha com erro acionável + link.** Quando o usuário liga `tlsEnabled` de um servidor e **não existe** bundle no TLS manager, a ação é recusada (toggle permanece off) com mensagem tipo `No TLS certificate yet — create one in the TLS manager` e um deep-link para a página. Nunca auto-gerar nesse momento. |
| 4 | **Certificado existe mas não cobre a identidade de bind do servidor → warning, não erro.** Ex: cert só com loopback e o usuário liga TLS no Web (IP Tailscale). Permitir ligar (o acesso real pode ser por um nome MagicDNS coberto), mostrar warning no status/UI e oferecer **Regenerate** para incluir a identidade que falta. |
| 5 | **Subsistema compartilhado, um cert para os três.** Pacote `internal/infrastructure/servertls`, perfil único. SANs do self-signed = **união** das identidades relevantes (localhost, `127.0.0.1`, `::1`, IP Tailscale detectado, hostname/MagicDNS local). Regenerar inclui a união das identidades dos servidores com TLS ligado. |
| 6 | **Modos: `self_signed` e `custom`.** `self_signed` é gerenciado pelo app; `custom` é cadeia PEM + chave PEM enviadas pelo usuário. Sem PFX/PKCS#12, sem chave PEM criptografada na v1. `tailscale cert` fica fora do v1 (ver §11). |
| 7 | **A chave privada NUNCA vai pro `config.json`, vault, logs ou DTOs.** O Web Access pode subir **antes** do unlock do vault, então o material TLS fica em **arquivo no data dir** (`<appconfig.BaseDir()>/server-tls/`), dir `0700`, bundle `0600`, escrita temp+rename, `Chmod` best-effort. Esse trade-off (chave protegida por permissão de arquivo, não pelo vault) é documentado no código. Não persistir paths em config/DTO. |
| 8 | **Bundle atômico por modo:** `server-tls/self-signed.pem` ou `server-tls/custom.pem` (cadeia + chave concatenadas). Trocar/regenerar reescreve atômico; um par inválido **nunca** substitui um bundle válido. |
| 9 | **Upload custom só instala após validação completa.** Erro de parsing, chave incompatível, fora de validade, sem SAN, ou EKU incompatível → mantém o cert/servidores atuais intactos. Nunca incluir PEM/chave em erro ou log. |
| 10 | **Trocar/regenerar o cert reinicia apenas os servidores que estão rodando COM TLS**, sem mexer em autostart, bind, porta, TTL, bearer token ou chave de sessão. A troca de cert **não** invalida sessões do Web nem tokens MCP/REST. |
| 11 | **Self-signed não remove o aviso do navegador.** A UI deve explicar: ele cifra a conexão, mas para tirar o warning o usuário precisa confiar o cert localmente ou usar um cert próprio confiável e acessar por um nome coberto pelos SANs. |
| 12 | **TLS mínimo 1.2, defaults modernos do Go.** Sem lista manual de cifras, sem HSTS, sem redirect HTTP→HTTPS, sem trust-store automático. |
| 13 | **Nenhum tool do agente lê/instala/exporta cert ou chave.** Sem action `aw`/MCP/REST para isso; sem exportação pela UI. Os métodos Wails de gestão do cert entram na denylist do bridge web se a política exigir, mas a página TLS manager autenticada funciona tanto na janela Wails quanto no browser remoto. |

## 3. Estado atual que deve ser preservado

- `webserver.NewServer(opts Options)` serve via `httpServer.Serve(listener)` e
  aplica `peerGuard` antes dos handlers; sobe e permanece com vault locked.
  `domain.WebServerConfig` (em `config.json`) já usa `*bool`/`*int` para
  round-trip de zero-value.
- `mcpserver.NewServer(backend, cfg Config)` e
  `restserver.NewServer(backend, cfg Config)` fazem bind loopback com bearer
  token; settings ficam no vault (`application.RestServerSettings`/
  `APIServerSettings`); sobem só **pós-unlock**.
- Lifecycle Web compartilhado por GUI e daemon em `internal/appcore/app_web.go`;
  `cmd/awd` usa o mesmo `webserver`. MCP/REST em `app_mcp.go`/`app_rest.go`.
- Tailscale continua o bind recomendado do Web e recusando peers fora de
  `100.64.0.0/10`. TLS não relaxa essa regra.

## 4. Modelo e contratos

### 4.1 Configuração não secreta (perfil TLS compartilhado)

Em `config.json` (legível pré-unlock), adicionar um perfil compartilhado, p.ex.
`domain.ServerTLSConfig{ Mode string }` com `TLSModeSelfSigned = "self_signed"` e
`TLSModeCustom = "custom"`, mais `ModeOrDefault()` (default `self_signed` quando
ausente — mas note: ter modo não implica existir bundle; "existe cert" é checado
no material, ver 4.2). Estender `appconfig.Store` + use cases para carregar/
validar/salvar o modo; modo desconhecido é rejeitado.

**Toggle por servidor (`tlsEnabled`, default false):**
- Web: novo campo `TLS *bool` em `domain.WebServerConfig` (config.json, `*bool`
  pelo mesmo motivo de zero-value do `Enabled`).
- MCP/REST: novo flag `tlsEnabled` junto das settings existentes no vault
  (`RestServerSettings`/`APIServerSettings`) — eles só sobem pós-unlock, então
  vault serve. Cada servidor lê o próprio flag no start.

### 4.2 Material TLS pré-unlock — `internal/infrastructure/servertls/`

Criar `ports.TLSMaterialStore` (domínio) e o adapter `servertls` (infra),
responsável por:

- gerar/carregar/inspecionar o bundle self-signed;
- validar e instalar atomicamente o bundle custom;
- reportar **existe bundle?** (usado no gating do toggle, decisão #3) e se o
  bundle **cobre** uma dada identidade (decisão #4);
- devolver ao use case um `tls.Certificate`/`*tls.Config` interno para os
  servidores;
- devolver **metadados sanitizados** para status;
- **nunca** devolver a chave por DTO/JSON/log.

`App` recebe o adapter na composição e chama apenas use cases de
`internal/application` (`web_tls.go`/`server_tls.go`); nada de filesystem ou
`crypto/x509` em `internal/appcore`. O tipo que carrega a chave tem `json:"-"` e
jamais cruza a fronteira Wails. Limite de entrada: **≤1 MiB para a cadeia** e
**≤1 MiB para a chave**, validado no backend (a bridge aceita payload maior por
causa de attachments).

### 4.3 Certificado self-signed

Gerar com: chave **ECDSA P-256**, serial aleatório, SHA-256,
`NotBefore = now-5min`, validade 365 dias, `KeyUsageDigitalSignature` +
`ExtKeyUsageServerAuth`, subject legível (`CN=Agent Workspace Local`), e **SANs
= união** de `localhost`, hostname local quando disponível, loopbacks (`127.0.0.1`,
`::1`), IPs locais relevantes e o **IP Tailscale detectado**. Nunca só
`CommonName` (navegadores exigem SAN). Para bind `0.0.0.0`, incluir os IPs reais
detectados, não `0.0.0.0`. Sem consulta de rede externa nem IP público inventado.

Regeneração é **manual** (botão). Não auto-regenerar no start; mas o status deve
sinalizar quando o bundle está inválido/expirando (≤30 dias) ou não cobre a
identidade de bind de um servidor com TLS ligado, para a UI oferecer Regenerate.

### 4.4 Certificado custom

Upload recebe dois textos: cadeia PEM (leaf primeiro, intermediários opcionais)
e chave PEM (RSA/ECDSA/Ed25519, não criptografada). Antes de persistir:

1. confirmar blocos PEM de cert e de chave;
2. `tls.X509KeyPair` (prova que chave casa com o leaf);
3. parsear o leaf e exigir SAN DNS ou IP;
4. exigir `NotBefore <= now < NotAfter`;
5. se EKU presente, exigir `ServerAuth` ou `Any`;
6. derivar metadados + fingerprint SHA-256 do leaf;
7. concatenar cadeia+chave e gravar o bundle atomicamente.

Não validar a cadeia contra roots do sistema como condição de instalação (CA
privada e self-signed próprio são válidos; o navegador decide confiança). Não
cobrir o host sugerido → instala com **warning**, não erro.

### 4.5 Status público

DTO sanitizado, p.ex. `dto.ServerTLSStatus`:

```text
mode, hasCertificate, ready, selfSigned, subject, issuer,
notBefore, notAfter, fingerprintSHA256, dnsNames[], ipAddresses[],
expiringSoon, coverageWarning, error
```

Datas em RFC 3339. **Sem** PEM, chave, ou path do bundle. Cada `*ServerStatus`
(web/mcp/rest) ganha `tlsEnabled bool` e, quando ligado, referencia este status
compartilhado (ou um `coverageWarning` por servidor quando o cert não cobre o
bind daquele servidor).

### 4.6 Métodos da interface (Wails)

No TLS manager:
- `CreateSelfSignedCertificate()` — gera o self-signed (decisão #2), modo
  `self_signed`. Reinicia servidores que estiverem rodando com TLS.
- `RegenerateSelfSignedCertificate()` — nova chave+cert (união de SANs atual).
- `InstallCustomCertificate(certPEM, keyPEM string)` — valida, instala, modo
  `custom`.
- `GetServerTLSStatus()` — status sanitizado.

Por servidor (espelhando os toggles existentes de autostart/port):
- `SetWebServerTLSEnabled(bool)`, `SetMcpServerTLSEnabled(bool)`,
  `SetRestServerTLSEnabled(bool)` — se `true` e **não há bundle** → retornar erro
  acionável com pointer para o TLS manager (decisão #3), sem ligar. Se houver
  bundle, persistir o flag e reiniciar aquele servidor.

Todos retornam status sanitizado. Nenhuma action `aw`/tool de agente para
cert/chave (decisão #13).

## 5. Servidor e lifecycle

1. Cada `start{Web,Mcp,Rest}Server()` lê o `tlsEnabled` do seu servidor. Se on:
   carregar o `tls.Certificate` do `servertls` **antes** de abrir o listener; se
   o bundle não existir/for inválido → **falha fechada** (não cair para
   plaintext) com erro sanitizado no `*ServerStatus.Error`.
2. **Os três servidores ganham um caminho TLS no serve:** trocar
   `Serve(listener)` por `ServeTLS`/`tls.NewListener(listener, tlsCfg)` quando o
   `*tls.Config` for não-nil; nil = plaintext (inalterado). Para o `webserver`,
   passar `TLS *tls.Config` em `Options`; para `mcpserver`/`restserver`, passar
   em `Config`. O `*tls.Config` carrega o `tls.Certificate`; `MinVersion = 1.2`.
3. `Server.URL()` reflete o esquema: `https://` quando TLS ligado, senão `http://`.
4. `/healthz`, login/setup, bridge, SSE e assets (Web) ficam todos no mesmo
   listener; sem exceção plaintext para healthcheck.
5. `peerGuard`/loopback, CORS, rate limiter, auth, session epoch, bearer token e
   lifecycle lock/unlock permanecem com a mesma semântica.
6. Web mantém o lifecycle independente do lock; ligar/desligar TLS no Web é
   tratado como troca de config que reinicia o servidor se estiver rodando.
7. Atualizar URLs calculados (`WebAccessPage`, MCP/REST pages, `cmd/awd`/tray,
   stdout do `awd`, docs) para refletir o esquema conforme o toggle.

## 6. UX

### 6.1 TLS manager (Settings/Security, novo)

Página/subsistema dedicado, separado dos cards de cada servidor:
- **Estado do certificado:** modo ativo, se existe bundle, subject, issuer,
  validade, SANs, fingerprint SHA-256. Expirado/ausente = erro; ≤30 dias =
  warning.
- **Self-signed:** botão **Create self-signed certificate** (quando não há
  bundle) e **Regenerate** (com confirmação curta de que o navegador pode pedir
  nova aprovação). Texto da decisão #11.
- **Custom:** dois `<input type="file">` (`Certificate chain .pem/.crt` e
  `Private key .pem/.key`), lidos com `File.text()` no frontend (funciona também
  pelo Web Access remoto, sem native dialog). Mostrar só os nomes dos arquivos,
  nunca o conteúdo. Botão **Validate and install** só com os dois. Limpar inputs
  após sucesso/erro; mostrar erro sanitizado e manter o cert atual.

### 6.2 Cards dos servidores (Web/MCP/REST)

Cada página de servidor ganha um toggle **Enable HTTPS (TLS)** default off. Ao
ligar sem cert → mostrar o erro acionável com **link para o TLS manager**
(decisão #3). Quando ligado mas o cert não cobre o bind daquele servidor →
mostrar o `coverageWarning` (decisão #4). A URL exibida passa a `https://`.
Reescrever avisos de "plain HTTP" para refletir o estado real (com/sem TLS).

## 7. Migração e compatibilidade

- Não-destrutiva: ausência de `tls*` = TLS off em todos (estado atual). Nada
  muda até o usuário criar um cert e ligar um toggle.
- Bookmarks/clientes `http://` continuam válidos enquanto o toggle do servidor
  estiver off. Ligar TLS num servidor quebra os clientes plaintext dele
  (esp. MCP/REST programáticos, que verificam cert) — documentar e exibir a nova
  URL. Migração é explícita e por servidor.
- GUI e `awd` compartilham `config.json` + bundle no mesmo `BaseDir`; ambos
  apresentam o mesmo certificado.
- Comportamento Win/macOS/Linux equivalente; em Windows não prometer semântica
  POSIX de `0600`, usar o data dir do usuário com escrita restritiva best-effort.

## 8. Fases de implementação

### Fase 1 — domínio, `servertls` storage e validação
Perfil/modo em config + toggles por servidor; porta `TLSMaterialStore` + adapter
`servertls` (bundle atômico, geração self-signed, validação custom, "existe?"/
"cobre?"); DTO sanitizado; testes unitários.
**Aceite:** material preparável pré-unlock; nenhuma chave em config/status/log;
par inválido nunca substitui bundle válido; "existe cert" e "cobre identidade"
corretos.

### Fase 2 — serve TLS nos três servidores
`*tls.Config` opcional em `webserver.Options` e `mcpserver`/`restserver.Config`;
caminho `ServeTLS`/`tls.NewListener` quando não-nil; `URL()` por esquema; ligar
sem cert falha fechado; ligar com cert reinicia só aquele servidor.
**Aceite:** HTTPS responde quando ligado; plaintext na porta HTTPS falha no
handshake; off = comportamento atual idêntico; Web continua subindo com vault
locked.

### Fase 3 — bindings e UI
Métodos Wails (manager + por servidor), service wrappers, página TLS manager e
toggle nos cards Web/MCP/REST; upload por file inputs; erro+link ao ligar sem
cert; warnings; URLs `https://`. Gerar bindings pelo fluxo normal.
**Aceite:** criar self-signed/custom; ligar TLS por servidor; erro acionável sem
cert; warning de cobertura; URL correta por toggle.

### Fase 4 — documentação e regressão
Atualizar `docs/SELFCODE.md` e comentários; varrer URLs afetadas (sem mexer em
OAuth callbacks, Vite, ou testes não relacionados); gate completo.

## 9. Testes obrigatórios

### Backend
- modo: ausente→`self_signed`; válidos persistem; desconhecido rejeitado.
- self-signed: par ECDSA P-256, ServerAuth, SANs-união (inclui IP Tailscale
  detectado), janela de validade, fingerprint.
- "existe bundle?" e "cobre identidade X?" corretos (loopback-only não cobre IP
  tailnet; após regenerate com a identidade, cobre).
- custom: par válido + cadeia aceitos; mismatch, PEM quebrado, expirado/futuro,
  sem SAN, EKU incompatível, chave criptografada e payload >1 MiB recusados;
  falha não altera o bundle anterior.
- bundle/dir com modos restritivos onde a plataforma permite; temp não deixa
  órfão após erro.
- config/status não contêm chave/PEM/path.
- **gating:** `Set*ServerTLSEnabled(true)` sem bundle retorna erro acionável e
  não liga; com bundle liga e reinicia só aquele servidor; cert sem cobrir o
  bind liga com `coverageWarning`.
- integração: cliente HTTPS alcança o servidor ligado; plaintext na porta TLS
  falha; `URL()` reflete o esquema; peer guard/loopback/bearer continuam sob TLS;
  Web sobe com vault locked; troca de cert reinicia só servidores TLS-on e não
  invalida sessões/tokens.

### Frontend
- TLS manager: create/regenerate/install chamam os bindings certos sem reter/
  renderizar o conteúdo da chave; status (modo, validade, SANs, fingerprint)
  exibido; install só com os dois arquivos; sucesso limpa inputs; erro
  sanitizado mantém o cert atual.
- cards: toggle default off; ligar sem cert mostra erro + link pro TLS manager;
  warning de cobertura; URL `https://` quando ligado; textos de plaintext
  reescritos conforme o estado.

## 10. Arquivos esperados (guia, não obrigação de nomes)

- `internal/domain/server_tls.go` (novo), `internal/domain/web_server.go`
  (campo `TLS *bool`), `internal/domain/ports/server_tls.go` (novo),
  `internal/domain/ports/app_settings.go`
- `internal/application/server_tls.go` (novo), `internal/application/web_settings.go`,
  `internal/application/api_servers.go` (flags MCP/REST)
- `internal/infrastructure/servertls/*` (novo)
- `internal/infrastructure/appconfig/webserver.go` / `config.go`
- `internal/infrastructure/webserver/server.go` (Options.TLS + ServeTLS)
- `internal/infrastructure/mcpserver/server.go` e
  `internal/infrastructure/restserver/server.go` (Config.TLS + ServeTLS)
- `internal/appcore/app.go`, `app_web.go`, `app_mcp.go`, `app_rest.go`
- `internal/dto/results.go`
- `cmd/awd` (URLs/tray)
- `frontend/src/modules/settings/pages/` (TLS manager + cards Web/MCP/REST),
  `frontend/src/services/` (wrappers)
- testes focados correspondentes e `docs/SELFCODE.md`

## 11. Fora de escopo

- `tailscale cert` / ACME / Let's Encrypt / emissão por CA / renovação automática
  (modo futuro recomendado para tirar o aviso do navegador na tailnet).
- PKCS#12/PFX e chave PEM protegida por senha.
- Instalação automática do self-signed no trust store do SO/navegador.
- Redirect HTTP→HTTPS, HSTS, reverse proxy embutido.
- HTTPS obrigatório em qualquer servidor.
- Exposição de cert/chave por action `aw`, MCP, REST ou pela UI.

## 12. Gate e disciplina de entrega

Executar ao final de cada fase:

```sh
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/go/bin:$PATH"
golangci-lint run ./...
go test ./...
go run ./tools/buildgate
cd frontend && npm run build:frontend
```

Seguir `docs/AGENTS.md`: trabalhar em `main`, commits pequenos, nunca
`git add -A`, preservar alterações de outros agentes e avisar que mudanças de
backend exigem reabrir o app.
