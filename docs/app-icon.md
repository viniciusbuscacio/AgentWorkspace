# Agent Workspace app icon

> Última atualização: 2026-06-24

## TL;DR obrigatório

O ícone correto do Agent Workspace **NÃO é** o `W` do Wails e **NÃO deve ser recriado manualmente**.

A fonte correta é este PNG:

```txt
%USERPROFILE%\Desktop\New folder\Agent Workspace icon.png
```

Antes de mexer em ícone, validar esse arquivo. Se ele existir, usar ele. Não desenhar um óculos genérico.

## Fonte correta do ícone

O ícone correto do Agent Workspace é o PNG:

```txt
%USERPROFILE%\Desktop\New folder\Agent Workspace icon.png
```

Esse PNG deve ser copiado para o asset canônico do Wails:

```txt
build/appicon.png
```

Hash SHA-256 validado em 2026-06-24:

```txt
201871d807935426890511d84bc2a2c9292fca42cd3326e3ab3aa6f74236b983
```

Validação:

```powershell
Get-FileHash "%USERPROFILE%\Desktop\New folder\Agent Workspace icon.png" -Algorithm SHA256
Get-FileHash "%USERPROFILE%\aw\build\appicon.png" -Algorithm SHA256
```

Os hashes devem ser iguais.

## Assets gerados para Windows

O build Windows usa:

```txt
build/windows/icon.ico
```

Também mantemos o nome legado usado pelo atalho pinado da taskbar:

```txt
build/windows/agent-workspace-glasses-v2.ico
```

Ambos devem ser gerados a partir de `build/appicon.png` e precisam ser idênticos entre si.

Hash SHA-256 validado em 2026-06-24 para os `.ico` corretos:

```txt
7603632dd61c89b0ae152580f8d9bf3f330448f875e83512491801d0c3302e2e
```

Validação:

```powershell
Get-FileHash "%USERPROFILE%\aw\build\windows\icon.ico" -Algorithm SHA256
Get-FileHash "%USERPROFILE%\aw\build\windows\agent-workspace-glasses-v2.ico" -Algorithm SHA256
```

Os hashes devem ser iguais.

## Build correto

Depois de atualizar `build/appicon.png` e `build/windows/icon.ico`, rodar:

```powershell
cd %USERPROFILE%\aw
wails build
```

O executável principal fica em:

```txt
%USERPROFILE%\aw\build\bin\Agent Workspace.exe
```

Se precisar também do `bin4`, copiar o EXE compilado:

```powershell
New-Item -ItemType Directory -Force "%USERPROFILE%\aw\build\bin4" | Out-Null
Copy-Item -Force "%USERPROFILE%\aw\build\bin\Agent Workspace.exe" "%USERPROFILE%\aw\build\bin4\Agent Workspace.exe"
```

## Como validar o ícone embutido no EXE

**Não confiar só no ícone que o Explorer mostra**, porque o Windows pode mostrar cache antigo (`W`) mesmo quando o EXE já está certo.

Validar extraindo o ícone associado do próprio EXE:

```powershell
Add-Type -AssemblyName System.Drawing
$exe = "%USERPROFILE%\aw\build\bin\Agent Workspace.exe"
$out = "%USERPROFILE%\aw\build\bin\Agent Workspace.associated-icon.png"
$ico = [System.Drawing.Icon]::ExtractAssociatedIcon($exe)
$bmp = $ico.ToBitmap()
$bmp.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
$ico.Dispose()
```

Abrir/inspecionar:

```txt
%USERPROFILE%\aw\build\bin\Agent Workspace.associated-icon.png
```

Resultado esperado: ícone de óculos correto, não o `W`.

Para `bin4`:

```powershell
Add-Type -AssemblyName System.Drawing
$exe = "%USERPROFILE%\aw\build\bin4\Agent Workspace.exe"
$out = "%USERPROFILE%\aw\build\bin4\Agent Workspace.associated-icon.png"
$ico = [System.Drawing.Icon]::ExtractAssociatedIcon($exe)
$bmp = $ico.ToBitmap()
$bmp.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
$ico.Dispose()
```

## Explorer/Taskbar mostrando `W` mesmo depois do build

Se o app abre com o ícone correto, e o `ExtractAssociatedIcon` também retorna o óculos, mas o Explorer/taskbar ainda mostra `W`, é cache do Windows.

Limpar cache e reiniciar Explorer:

```powershell
Stop-Process -Name explorer -Force
Start-Sleep -Seconds 2
Remove-Item "$env:LOCALAPPDATA\IconCache.db" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\Microsoft\Windows\Explorer\iconcache*" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\Microsoft\Windows\Explorer\thumbcache*" -Force -ErrorAction SilentlyContinue
Start-Process explorer.exe
```

Se ainda persistir, fazer unpin/repin do atalho ou reiniciar o Windows.

## Atalho pinado da taskbar

O atalho pinado fica em:

```txt
%APPDATA%\Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar\Agent Workspace.lnk
```

Ele deve apontar para:

```txt
%USERPROFILE%\aw\build\bin\Agent Workspace.exe
```

E o `IconLocation` deve apontar para:

```txt
%USERPROFILE%\aw\build\windows\agent-workspace-glasses-v2.ico,0
```

Comando para corrigir:

```powershell
$lnk = Join-Path $env:APPDATA "Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar\Agent Workspace.lnk"
$shell = New-Object -ComObject WScript.Shell
$sc = $shell.CreateShortcut($lnk)
$sc.TargetPath = "%USERPROFILE%\aw\build\bin\Agent Workspace.exe"
$sc.IconLocation = "%USERPROFILE%\aw\build\windows\agent-workspace-glasses-v2.ico,0"
$sc.Save()
```

## Sobre o “SVG” / ícone vetorial do fundo do chat

O ícone grande que aparece no fundo de um chat novo **não é um arquivo SVG no repo**.
Ele é renderizado como glyph da fonte Google Material Symbols:

```txt
eyeglasses_2
```

Código:

```txt
frontend/src/modules/chat/MessageList.tsx
```

Trecho:

```tsx
<span className="material-symbols-outlined empty-chat-logo" role="img" aria-label="Agent Workspace">
  eyeglasses_2
</span>
```

Estilo:

```txt
frontend/src/theme/aw-chat.css
```

Fonte carregada em:

```txt
frontend/src/theme/globals.css
```

```css
@import url("https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL,GRAD@48,400,0,0") layer(vendor);
```

Ou seja: o fundo do chat é vetorial porque vem de uma fonte (`Material Symbols Outlined`), não porque exista um `.svg` local.

## Build guard hardcoded

Existe um teste TypeScript que quebra o build se alguém trocar os ícones pelo `W` do Wails ou por um óculos genérico:

```txt
frontend/src/app-icon.guard.test.ts
```

Ele valida hashes SHA-256 hardcoded de:

```txt
build/appicon.png
build/windows/icon.ico
build/windows/agent-workspace-glasses-v2.ico
```

Como `wails build` chama `npm run build`, que passa pelo buildgate e roda `npm run build:frontend`, e `build:frontend` roda `npm run test`, esse guard bloqueia a compilação quando os assets estão errados.

Teste isolado:

```powershell
cd %USERPROFILE%\aw\frontend
npx vitest run src/app-icon.guard.test.ts
```

## Regra operacional para agentes

- Não gerar ícone “óculos” genérico manualmente.
- Não usar o `W` do Wails.
- Para alterar o ícone do app, começar sempre pelo PNG fonte: `%USERPROFILE%\Desktop\New folder\Agent Workspace icon.png`.
- Copiar esse PNG para `build/appicon.png`.
- Gerar `build/windows/icon.ico` e `build/windows/agent-workspace-glasses-v2.ico` a partir desse PNG.
- Se os hashes mudarem intencionalmente, atualizar também `frontend/src/app-icon.guard.test.ts`.
- Rodar `wails build` para embutir o ícone no executável.
- Validar com `ExtractAssociatedIcon` antes de acreditar no Explorer.
- Se Explorer/taskbar ainda mostrar `W`, limpar cache/reiniciar Explorer.
