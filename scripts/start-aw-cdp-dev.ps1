param(
  [int]$CdpPort = 9225,
  [string]$AppUrl = "http://127.0.0.1:34115",
  [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"

$repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$buildDir = Join-Path $repo "build"
$stdoutLog = Join-Path $buildDir "aw-wails-dev.log"
$stderrLog = Join-Path $buildDir "aw-wails-dev.err.log"
$edgeProfile = Join-Path $buildDir "cdp-profile"

New-Item -ItemType Directory -Force -Path $buildDir | Out-Null

function Test-HttpOk([string]$Url) {
  try {
    $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2
    return ($response.StatusCode -ge 200 -and $response.StatusCode -lt 400)
  } catch {
    return $false
  }
}

if (-not (Test-HttpOk $AppUrl)) {
  Write-Output "Starting Wails dev server..."
  if (Test-Path $stdoutLog) { Remove-Item -LiteralPath $stdoutLog -Force -ErrorAction SilentlyContinue }
  if (Test-Path $stderrLog) { Remove-Item -LiteralPath $stderrLog -Force -ErrorAction SilentlyContinue }

  Start-Process `
    -FilePath "wails" `
    -ArgumentList @("dev", "-browser") `
    -WorkingDirectory $repo `
    -WindowStyle Hidden `
    -RedirectStandardOutput $stdoutLog `
    -RedirectStandardError $stderrLog
}

$deadline = (Get-Date).AddSeconds(60)
while ((Get-Date) -lt $deadline) {
  if (Test-HttpOk $AppUrl) { break }
  Start-Sleep -Milliseconds 500
}

if (-not (Test-HttpOk $AppUrl)) {
  Write-Error "Agent Workspace dev server did not become available at $AppUrl. Logs: $stdoutLog / $stderrLog"
}

if (-not $NoBrowser) {
  $cdpURL = "http://127.0.0.1:$CdpPort/json/version"
  if (Test-HttpOk $cdpURL) {
    Write-Output "CDP endpoint already available on port $CdpPort."
    Write-Output "Agent Workspace dev server: $AppUrl"
    Write-Output "CDP endpoint: http://127.0.0.1:$CdpPort"
    Write-Output "Smoke: node scripts/aw-cdp-smoke.mjs"
    exit 0
  }

  Write-Output "Starting Edge with CDP on port $CdpPort..."
  $edge = Get-Command "msedge.exe" -ErrorAction SilentlyContinue
  if (-not $edge) {
    $edgePath = "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
    if (Test-Path $edgePath) {
      $edge = [pscustomobject]@{ Source = $edgePath }
    }
  }
  if (-not $edge) {
    Write-Error "Could not find msedge.exe. Start a Chromium browser manually with --remote-debugging-port=$CdpPort."
  }

  New-Item -ItemType Directory -Force -Path $edgeProfile | Out-Null
  Start-Process `
    -FilePath $edge.Source `
    -ArgumentList @(
      "--remote-debugging-port=$CdpPort",
      "--user-data-dir=$edgeProfile",
      "--no-first-run",
      "--no-default-browser-check",
      $AppUrl
    ) `
    -WindowStyle Hidden

  $cdpDeadline = (Get-Date).AddSeconds(15)
  while ((Get-Date) -lt $cdpDeadline) {
    if (Test-HttpOk $cdpURL) { break }
    Start-Sleep -Milliseconds 500
  }

  if (-not (Test-HttpOk $cdpURL)) {
    Write-Error "CDP endpoint did not become available at http://127.0.0.1:$CdpPort"
  }
}

Write-Output "Agent Workspace dev server: $AppUrl"
Write-Output "CDP endpoint: http://127.0.0.1:$CdpPort"
Write-Output "Smoke: node scripts/aw-cdp-smoke.mjs"
