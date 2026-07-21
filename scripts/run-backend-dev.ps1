param(
  [string]$Addr = "127.0.0.1:9309",
  [string]$Token = "dev-token",
  [string]$DataDir = "",
  [string]$Edge = "",
  [int]$EdgePort = 0,
  [switch]$PermitAll,
  [switch]$Once,
  [switch]$Chat,
  [string]$VaultPassword = ""
)

$ErrorActionPreference = "Stop"

$repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($DataDir) -and -not $Chat) {
  $DataDir = Join-Path $env:TEMP "aw-harness-data"
}

function Get-AddrPort {
  try {
    return [int]($Addr -split ":")[-1]
  } catch {
    return 0
  }
}

function Stop-ExistingHarnessOnPort {
  $port = Get-AddrPort
  if ($port -le 0) { return }
  $connections = @(Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue)
  foreach ($connection in $connections) {
    $ownerPid = [int]$connection.OwningProcess
    if ($ownerPid -le 0) { continue }
    $proc = Get-CimInstance Win32_Process -Filter "ProcessId=$ownerPid" -ErrorAction SilentlyContinue
    $cmd = if ($proc) { [string]$proc.CommandLine } else { "" }
    if ($cmd -notmatch "awharness" -and $cmd -notmatch "cmd/awharness" -and $cmd -notmatch "cmd\\awharness") {
      Write-Warning "Port $port is busy by PID $ownerPid, but it does not look like awharness. Not stopping it."
      continue
    }
    Write-Output "Stopping stale backend harness on port $port (PID $ownerPid)..."
    Stop-Process -Id $ownerPid -Force -ErrorAction SilentlyContinue
  }
}

Stop-ExistingHarnessOnPort

$args = @(
  "run", "./cmd/awharness",
  "--addr", $Addr,
  "--token", $Token
)
if (-not [string]::IsNullOrWhiteSpace($DataDir)) {
  $args += @("--data-dir", $DataDir)
}
if (-not [string]::IsNullOrWhiteSpace($Edge)) {
  $args += @("--edge", $Edge)
}
if ($EdgePort -gt 0) {
  $args += @("--edge-port", "$EdgePort")
}
if ($PermitAll) {
  $args += "--permit-all"
}
if ($Chat) {
  $args += "--chat"
}
if (-not [string]::IsNullOrWhiteSpace($VaultPassword)) {
  $args += @("--password", $VaultPassword)
}
if ($Once) {
  $args += "--once"
}

Write-Output "Starting AW backend harness..."
Write-Output "  repo: $repo"
if (-not [string]::IsNullOrWhiteSpace($DataDir)) {
  Write-Output "  data: $DataDir"
} else {
  Write-Output "  data: real app data dir"
}
Write-Output "  addr: $Addr"
Write-Output "  token: $Token"
if ($Chat) {
  Write-Output "  chat: enabled"
}
& go -C $repo @args
