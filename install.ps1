<#
.SYNOPSIS
    Aide Windows Installer
.DESCRIPTION
    Downloads the latest (or specified) Aide release and installs it to
    %USERPROFILE%\.local\bin\aide.exe.  Adds the directory to your user PATH
    so that `aide` is available from any terminal.
.PARAMETER Version
    Release version to install (without the 'v' prefix).  Defaults to "latest".
.PARAMETER Arch
    Target architecture: "amd64" (default) or "arm64".
.EXAMPLE
    powershell -c "irm https://raw.githubusercontent.com/accreation/aide/main/install.ps1 | iex"
.EXAMPLE
    .\install.ps1 -Version "0.2.0"
.EXAMPLE
    .\install.ps1 -Arch arm64
#>

param(
    [string]$Version = "latest",
    [ValidateSet("amd64", "arm64")]
    [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"

$BinDir   = "$env:USERPROFILE\.local\bin"
$BinName  = "aide.exe"
$DestPath = Join-Path $BinDir $BinName

# ── Determine download URL ──────────────────────────────────────────
if ($Version -eq "latest") {
    $DownloadUrl = "https://github.com/accreation/aide/releases/latest/download/aide-windows-$Arch.exe.zip"
} else {
    $DownloadUrl = "https://github.com/accreation/aide/releases/download/v$Version/aide-windows-$Arch.exe.zip"
}

# ── Create bin directory ────────────────────────────────────────────
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

# ── Download ────────────────────────────────────────────────────────
Write-Host "Downloading aide $Version ($Arch)..." -ForegroundColor Cyan
Write-Host "  $DownloadUrl"

$TempZip = Join-Path $env:TEMP "aide-install-$PID.zip"
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TempZip -UseBasicParsing
} catch {
    if ($_.Exception.Response.StatusCode -eq 404) {
        Write-Host "Error: Release not found. Check the version and architecture." -ForegroundColor Red
        Write-Host "Available releases: https://github.com/accreation/aide/releases" -ForegroundColor Yellow
    }
    throw
}

# ── Extract ─────────────────────────────────────────────────────────
Write-Host "Installing to $DestPath..."

# The zip contains a single file named aide-windows-<arch>.exe
$ExtractedName = "aide-windows-$Arch.exe"
$ExtractDir    = Join-Path $env:TEMP "aide-extract-$PID"

Expand-Archive -Path $TempZip -DestinationPath $ExtractDir -Force

$ExtractedPath = Join-Path $ExtractDir $ExtractedName
if (-not (Test-Path $ExtractedPath)) {
    # Fallback: look for any .exe in the extracted directory
    $exe = Get-ChildItem -Path $ExtractDir -Filter "*.exe" | Select-Object -First 1
    if (-not $exe) {
        throw "No executable found in the downloaded archive."
    }
    $ExtractedPath = $exe.FullName
}

Move-Item -Path $ExtractedPath -Destination $DestPath -Force

# ── Cleanup ─────────────────────────────────────────────────────────
Remove-Item $TempZip -Force -ErrorAction SilentlyContinue
Remove-Item $ExtractDir -Recurse -Force -ErrorAction SilentlyContinue

# ── Add to PATH ─────────────────────────────────────────────────────
$CurrentUserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not $CurrentUserPath) { $CurrentUserPath = "" }

if ($CurrentUserPath -notlike "*$BinDir*") {
    Write-Host "Adding $BinDir to user PATH..."
    $NewPath = if ($CurrentUserPath) { "$CurrentUserPath;$BinDir" } else { $BinDir }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")

    # Refresh PATH in the current session
    $env:Path = "$env:Path;$BinDir"
}

# ── Done ────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "Aide installed successfully!" -ForegroundColor Green
Write-Host "  Version: $(& $DestPath --version 2>$null)"
Write-Host "  Location: $DestPath"
Write-Host ""
Write-Host "Restart your terminal or run 'refreshenv' for PATH changes to take effect."
Write-Host "Then try: aide --help"
