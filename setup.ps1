#Requires -Version 5.1
<#
.SYNOPSIS
    Setup script for GoPro Clip Extractor GUI
.DESCRIPTION
    Checks and installs all required dependencies (Go, FFmpeg, MSYS2/GCC),
    then builds the application.
.EXAMPLE
    .\setup.ps1
    .\setup.ps1 -SkipBuild
    .\setup.ps1 -RunAfterBuild
#>

param(
    [switch]$SkipBuild,
    [switch]$RunAfterBuild
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$GuiDir = Join-Path $ScriptDir "gui"
$ExePath = Join-Path $GuiDir "gopro-gui.exe"

function Write-Step($message) {
    Write-Host "`n==> $message" -ForegroundColor Cyan
}

function Write-Success($message) {
    Write-Host "    [OK] $message" -ForegroundColor Green
}

function Write-Warning($message) {
    Write-Host "    [!] $message" -ForegroundColor Yellow
}

function Write-Error($message) {
    Write-Host "    [X] $message" -ForegroundColor Red
}

function Test-Command($command) {
    $null = Get-Command $command -ErrorAction SilentlyContinue
    return $?
}

function Install-WithWinget($packageId, $packageName) {
    Write-Host "    Installing $packageName via winget..." -ForegroundColor Yellow
    $output = winget install --id $packageId -e --accept-package-agreements --accept-source-agreements 2>&1 | Out-String
    # Exit code 0 = success, -1978335189 = already installed
    if ($LASTEXITCODE -eq 0) {
        Write-Success "$packageName installed"
    } elseif ($output -match "already installed" -or $output -match "No available upgrade") {
        Write-Success "$packageName is already installed"
    } else {
        Write-Host $output -ForegroundColor Red
        throw "Failed to install $packageName"
    }
    return $true
}

function Refresh-Path {
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
}

# Header
Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host "  GoPro Clip Extractor - Setup Script  " -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta

# Check for winget
Write-Step "Checking for winget package manager"
if (-not (Test-Command "winget")) {
    Write-Error "winget is not available. Please install App Installer from the Microsoft Store."
    exit 1
}
Write-Success "winget is available"

# Track what we installed (need PATH refresh)
$needsPathRefresh = $false

# Check/Install Go
Write-Step "Checking for Go"
$goPath = "C:\Program Files\Go\bin\go.exe"
if (Test-Path $goPath) {
    $goVersion = & $goPath version 2>$null
    Write-Success "Go is installed: $goVersion"
} elseif (Test-Command "go") {
    $goVersion = go version 2>$null
    Write-Success "Go is installed: $goVersion"
} else {
    Write-Warning "Go is not installed"
    Install-WithWinget "GoLang.Go" "Go"
    $needsPathRefresh = $true
}

# Check/Install FFmpeg
Write-Step "Checking for FFmpeg"
$ffmpegInPath = Test-Command "ffmpeg"
# Check if FFmpeg is installed via winget (may not be in current PATH yet)
$wingetFFmpeg = winget list --id Gyan.FFmpeg 2>$null | Select-String "Gyan.FFmpeg"
if ($ffmpegInPath) {
    Write-Success "FFmpeg is available in PATH"
} elseif ($wingetFFmpeg) {
    Write-Success "FFmpeg is installed via winget"
} else {
    Write-Warning "FFmpeg is not installed"
    $null = Install-WithWinget "Gyan.FFmpeg" "FFmpeg"
    $needsPathRefresh = $true
}

# Check/Install MSYS2 and GCC
Write-Step "Checking for C compiler (GCC)"
$gccPath = "C:\msys64\mingw64\bin\gcc.exe"
if (Test-Path $gccPath) {
    $gccVersion = & $gccPath --version 2>$null | Select-Object -First 1
    Write-Success "GCC is installed: $gccVersion"
} elseif (Test-Command "gcc") {
    $gccVersion = gcc --version 2>$null | Select-Object -First 1
    Write-Success "GCC is available: $gccVersion"
} else {
    Write-Warning "GCC (C compiler) is not installed - required for GUI build"

    # Check if MSYS2 is installed
    if (-not (Test-Path "C:\msys64\usr\bin\bash.exe")) {
        Write-Host "    Installing MSYS2..." -ForegroundColor Yellow
        Install-WithWinget "MSYS2.MSYS2" "MSYS2"
    }

    # Install MinGW-w64 GCC via MSYS2
    Write-Host "    Installing MinGW-w64 GCC via MSYS2 pacman..." -ForegroundColor Yellow
    & C:\msys64\usr\bin\bash.exe -lc 'pacman -S --noconfirm mingw-w64-x86_64-gcc 2>&1' | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to install GCC via MSYS2"
    }
    Write-Success "GCC installed via MSYS2"
}

# Refresh PATH if we installed anything
if ($needsPathRefresh) {
    Write-Step "Refreshing PATH environment"
    Refresh-Path
    Write-Success "PATH refreshed"
}

# Build the application
if (-not $SkipBuild) {
    Write-Step "Building GoPro Clip Extractor GUI"

    # Determine Go executable path
    $goExe = if (Test-Path "C:\Program Files\Go\bin\go.exe") {
        "C:\Program Files\Go\bin\go.exe"
    } else {
        "go"
    }

    # Build using MSYS2 bash to ensure proper CGO environment
    $buildScript = @"
export PATH="/mingw64/bin:/c/Program Files/Go/bin:`$PATH"
export CGO_ENABLED=1
export GOPATH="/c/Users/$env:USERNAME/go"
export GOMODCACHE="`$GOPATH/pkg/mod"
export GOCACHE="`$GOPATH/cache"
cd /$(($GuiDir -replace '\\','/') -replace ':','')
go build -o gopro-gui.exe . 2>&1
"@

    Write-Host "    Compiling (this may take a minute on first build)..." -ForegroundColor Yellow
    $output = & C:\msys64\usr\bin\bash.exe -lc $buildScript 2>&1

    if (Test-Path $ExePath) {
        $fileInfo = Get-Item $ExePath
        $sizeMB = [math]::Round($fileInfo.Length / 1MB, 1)
        Write-Success "Build successful: gopro-gui.exe ($sizeMB MB)"
    } else {
        Write-Error "Build failed"
        Write-Host $output -ForegroundColor Red
        exit 1
    }
} else {
    Write-Step "Skipping build (--SkipBuild specified)"
}

# Summary
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Setup Complete!                       " -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "To run the application:" -ForegroundColor White
Write-Host "  $ExePath" -ForegroundColor Cyan
Write-Host ""

if ($RunAfterBuild -and (Test-Path $ExePath)) {
    Write-Step "Launching application"
    Start-Process $ExePath
}
