param(
  [switch]$Pull,
  [switch]$SkipTests,
  [switch]$StopRunning,
  [switch]$Run,
  [switch]$OpenFolder
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$exe = Join-Path $repo "build\bin\Agent Workspace.exe"

function Write-Step([string]$Message) {
  Write-Host ""
  Write-Host "==> $Message" -ForegroundColor Cyan
}

function Require-Command([string]$Name, [string]$InstallHint) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "$Name not found in PATH. $InstallHint"
  }
}

function Stop-AgentWorkspace {
  $names = @("Agent Workspace", "aw", "AW3")
  foreach ($name in $names) {
    $procs = @(Get-Process -Name $name -ErrorAction SilentlyContinue)
    foreach ($proc in $procs) {
      Write-Host "Stopping running process: $($proc.ProcessName) (PID $($proc.Id))"
      Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
  }
}

Push-Location $repo
try {
  Write-Host "Agent Workspace Windows build"
  Write-Host "Repo: $repo"

  Write-Step "Checking prerequisites"
  Require-Command "go" "Install Go and reopen the terminal."
  Require-Command "npm" "Install Node.js/npm and reopen the terminal."
  Require-Command "wails" "Install Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  if (-not $SkipTests) {
    Require-Command "golangci-lint" "Install it: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  }

  if ($Pull) {
    Write-Step "Updating main from origin"
    Require-Command "git" "Install Git and reopen the terminal."
    $dirty = (& git status --porcelain)
    if ($dirty) {
      throw "Working tree has local changes. Commit/stash them or run without -Pull."
    }
    & git fetch --prune origin
    & git pull --ff-only origin main
  }

  if ($StopRunning) {
    Write-Step "Stopping running Agent Workspace instances"
    Stop-AgentWorkspace
    Start-Sleep -Seconds 1
  }

  if ($SkipTests) {
    Write-Step "Building Windows executable quickly (skipping Go lint/tests)"
    $old = [Environment]::GetEnvironmentVariable("aw_SKIP_BUILD_GATE_TESTS", "Process")
    try {
      [Environment]::SetEnvironmentVariable("aw_SKIP_BUILD_GATE_TESTS", "1", "Process")
      & wails build -trimpath
    } finally {
      [Environment]::SetEnvironmentVariable("aw_SKIP_BUILD_GATE_TESTS", $old, "Process")
    }
  } else {
    Write-Step "Running full build gate (lint + tests + Wails build)"
    & go run ./tools/buildgate
  }

  if (-not (Test-Path $exe)) {
    throw "Build finished, but executable was not found: $exe"
  }

  Write-Step "Build complete"
  $item = Get-Item $exe
  Write-Host "Output: $exe" -ForegroundColor Green
  Write-Host ("Size: {0:N1} MB" -f ($item.Length / 1MB))
  Write-Host "Updated: $($item.LastWriteTime)"

  if ($OpenFolder) {
    Invoke-Item (Split-Path -Parent $exe)
  }

  if ($Run) {
    Write-Step "Starting Agent Workspace"
    Start-Process -FilePath $exe
  }
} finally {
  Pop-Location
}
