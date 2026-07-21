param(
  [string]$Addr = "127.0.0.1:9309",
  [string]$Token = "dev-token",
  [string]$DataDir = "",
  [string]$Edge = "",
  [int]$EdgePort = 0,
  [switch]$PermitAll,
  [switch]$Chat,
  [string]$VaultPassword = ""
)

$ErrorActionPreference = "Stop"

$repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if ([string]::IsNullOrWhiteSpace($DataDir) -and -not $Chat) {
  $DataDir = Join-Path $env:TEMP "aw-harness-data"
}

$logDir = Join-Path $repo "build"
$outLog = Join-Path $logDir "aw-harness-watch.out.log"
$errLog = Join-Path $logDir "aw-harness-watch.err.log"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null

$script:process = $null

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
    if ($ownerPid -le 0 -or ($script:process -and $ownerPid -eq $script:process.Id)) {
      continue
    }
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

function Stop-Harness {
  if ($script:process -and -not $script:process.HasExited) {
    Write-Output "Stopping backend harness (PID $($script:process.Id))..."
    try {
      Stop-Process -Id $script:process.Id -Force -ErrorAction Stop
      $script:process.WaitForExit(5000) | Out-Null
    } catch {
      Write-Warning "Could not stop backend harness: $($_.Exception.Message)"
    }
  }
  $script:process = $null
}

function Start-Harness {
  Stop-Harness
  Stop-ExistingHarnessOnPort
  if (Test-Path $outLog) { Remove-Item -LiteralPath $outLog -Force -ErrorAction SilentlyContinue }
  if (Test-Path $errLog) { Remove-Item -LiteralPath $errLog -Force -ErrorAction SilentlyContinue }

  $args = @(
    "-C", $repo,
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

  Write-Output "Starting backend harness..."
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
  Write-Output "  logs: $outLog / $errLog"

  $script:process = Start-Process `
    -FilePath "go" `
    -ArgumentList $args `
    -WorkingDirectory $repo `
    -WindowStyle Hidden `
    -RedirectStandardOutput $outLog `
    -RedirectStandardError $errLog `
    -PassThru

  Start-Sleep -Milliseconds 700
  if ($script:process.HasExited) {
    Write-Warning "Backend harness exited immediately with code $($script:process.ExitCode)."
    if (Test-Path $errLog) { Get-Content -Path $errLog -Tail 40 }
    if (Test-Path $outLog) { Get-Content -Path $outLog -Tail 40 }
    return
  }
  Write-Output "Backend harness running (PID $($script:process.Id)): http://$Addr/api/aw"
}

function Should-Restart([string]$Path) {
  if ([string]::IsNullOrWhiteSpace($Path)) { return $false }
  $full = [System.IO.Path]::GetFullPath($Path)
  if ($full -like "*\build\*") { return $false }
  if ($full -like "*\frontend\node_modules\*") { return $false }
  if ($full -like "*\.git\*") { return $false }
  $name = [System.IO.Path]::GetFileName($full)
  $ext = [System.IO.Path]::GetExtension($full)
  if ($ext -eq ".go") { return $true }
  if ($name -eq "go.mod" -or $name -eq "go.sum") { return $true }
  if ($name -eq "run-backend-dev.ps1" -or $name -eq "watch-backend-dev.ps1") { return $true }
  return $false
}

function Get-WatchedSignature {
  $entries = Get-ChildItem -LiteralPath $repo -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { Should-Restart $_.FullName } |
    Sort-Object FullName |
    ForEach-Object { "$($_.FullName)|$($_.Length)|$($_.LastWriteTimeUtc.Ticks)" }
  return ($entries -join "`n")
}

$watcher = New-Object System.IO.FileSystemWatcher
$watcher.Path = $repo
$watcher.IncludeSubdirectories = $true
$watcher.EnableRaisingEvents = $true

$script:lastRestart = [DateTime]::MinValue
$script:debounceMs = 800
$script:lastSignature = ""
$action = {
  $path = $Event.SourceEventArgs.FullPath
  if (-not (Should-Restart $path)) { return }
  $now = Get-Date
  if (($now - $script:lastRestart).TotalMilliseconds -lt $script:debounceMs) { return }
  $script:lastRestart = $now
  Write-Output "Change detected: $path"
  $script:lastSignature = Get-WatchedSignature
  Start-Harness
}

$subs = @()
$subs += Register-ObjectEvent -InputObject $watcher -EventName Changed -Action $action
$subs += Register-ObjectEvent -InputObject $watcher -EventName Created -Action $action
$subs += Register-ObjectEvent -InputObject $watcher -EventName Deleted -Action $action
$subs += Register-ObjectEvent -InputObject $watcher -EventName Renamed -Action $action

try {
  Start-Harness
  $script:lastSignature = Get-WatchedSignature
  Write-Output "Watching for backend changes. Press Ctrl+C to stop."
  while ($true) {
    Start-Sleep -Seconds 1
    $signature = Get-WatchedSignature
    if ($script:lastSignature -and $signature -ne $script:lastSignature) {
      $script:lastSignature = $signature
      $now = Get-Date
      if (($now - $script:lastRestart).TotalMilliseconds -ge $script:debounceMs) {
        $script:lastRestart = $now
        Write-Output "Change detected by polling."
        Start-Harness
      }
    }
    if ($script:process -and $script:process.HasExited) {
      Write-Warning "Backend harness exited with code $($script:process.ExitCode). Waiting for next file change."
      $script:process = $null
    }
  }
} finally {
  foreach ($sub in $subs) {
    Unregister-Event -SubscriptionId $sub.Id -ErrorAction SilentlyContinue
    Remove-Job -Id $sub.Id -Force -ErrorAction SilentlyContinue
  }
  $watcher.Dispose()
  Stop-Harness
}
