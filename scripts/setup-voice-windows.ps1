#requires -Version 5.1
<#
.SYNOPSIS
  Provision the local voice transcription engine (whisper.cpp) on Windows.

.DESCRIPTION
  Agent Workspace's local voice pipeline needs two native binaries that are NOT
  bundled with the app: whisper-cli (from whisper.cpp) and ffmpeg. ffmpeg is
  usually already on PATH (e.g. via winget Gyan.FFmpeg); this script provisions
  the missing whisper-cli.

  It downloads the official whisper.cpp Windows x64 CPU build, extracts
  whisper-cli.exe and its required DLLs into resources/voice/bin/windows-amd64,
  and sets the AW_WHISPER_CLI_PATH user environment variable so the installed
  app finds it regardless of where it is launched from. The whisper model
  (default: tiny) is downloaded automatically by the app on first use.

.PARAMETER Tag
  whisper.cpp release tag to install. Defaults to the latest release.

.PARAMETER NoEnv
  Skip setting the AW_WHISPER_CLI_PATH user environment variable.

.EXAMPLE
  .\scripts\setup-voice-windows.ps1
#>
[CmdletBinding()]
param(
  [string]$Tag = "",
  [switch]$NoEnv
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$targetDir = Join-Path $repoRoot "resources\voice\bin\windows-amd64"
$assetName = "whisper-bin-x64.zip"   # official CPU x64 Windows build

Write-Host "Agent Workspace voice setup (whisper.cpp)" -ForegroundColor Cyan
Write-Host "Repo:   $repoRoot"
Write-Host "Target: $targetDir`n"

# Resolve the download URL from the GitHub releases API (latest unless -Tag).
$api = if ($Tag) {
  "https://api.github.com/repos/ggml-org/whisper.cpp/releases/tags/$Tag"
} else {
  "https://api.github.com/repos/ggml-org/whisper.cpp/releases/latest"
}

Write-Host "==> Resolving release ($(if ($Tag) { $Tag } else { 'latest' }))"
$headers = @{ "User-Agent" = "aw-setup-voice"; "Accept" = "application/vnd.github+json" }
$release = Invoke-RestMethod -Uri $api -Headers $headers
$asset = $release.assets | Where-Object { $_.name -eq $assetName } | Select-Object -First 1
if (-not $asset) {
  throw "Could not find asset '$assetName' in whisper.cpp release '$($release.tag_name)'. Available: $(( $release.assets | ForEach-Object { $_.name }) -join ', ')"
}
Write-Host "    $($release.tag_name) -> $($asset.name) ($([math]::Round($asset.size / 1MB, 1)) MB)"

# Download + extract into a temp folder.
$work = Join-Path ([System.IO.Path]::GetTempPath()) ("aw-voice-" + [System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Force -Path $work | Out-Null
$zipPath = Join-Path $work $assetName
try {
  Write-Host "==> Downloading"
  Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zipPath -Headers @{ "User-Agent" = "aw-setup-voice" }

  Write-Host "==> Extracting"
  $extractDir = Join-Path $work "unzip"
  Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

  $cli = Get-ChildItem -Path $extractDir -Recurse -Filter "whisper-cli.exe" | Select-Object -First 1
  if (-not $cli) {
    throw "whisper-cli.exe not found inside $assetName (the release layout may have changed)."
  }
  # whisper-cli.exe needs its sibling DLLs; copy the whole folder it lives in.
  $binSrc = $cli.Directory.FullName
  New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
  Copy-Item -Path (Join-Path $binSrc "*") -Destination $targetDir -Recurse -Force

  $installedCli = Join-Path $targetDir "whisper-cli.exe"
  if (-not (Test-Path $installedCli)) {
    throw "Copy completed but $installedCli is missing."
  }
  Write-Host "==> Installed whisper-cli.exe + $((Get-ChildItem -Path $targetDir -Filter *.dll).Count) DLL(s)" -ForegroundColor Green

  if (-not $NoEnv) {
    Write-Host "==> Setting AW_WHISPER_CLI_PATH (user env)"
    [Environment]::SetEnvironmentVariable("AW_WHISPER_CLI_PATH", $installedCli, "User")
    Write-Host "    AW_WHISPER_CLI_PATH = $installedCli"
  }
}
finally {
  Remove-Item -Path $work -Recurse -Force -ErrorAction SilentlyContinue
}

# Sanity-check ffmpeg (the other half of the pipeline).
$ffmpeg = Get-Command ffmpeg -ErrorAction SilentlyContinue
if ($ffmpeg) {
  Write-Host "==> ffmpeg found on PATH: $($ffmpeg.Source)" -ForegroundColor Green
} else {
  Write-Host "==> ffmpeg NOT found on PATH. Install it (e.g. 'winget install Gyan.FFmpeg') or set AW_FFMPEG_PATH." -ForegroundColor Yellow
}

Write-Host "`nDone. Reopen Agent Workspace so it picks up the new environment variable, then try voice again." -ForegroundColor Cyan
Write-Host "The whisper model downloads automatically on first transcription."
