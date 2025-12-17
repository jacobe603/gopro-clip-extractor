# Extract clips using metadata from analyze_gopro_metadata.ps1
# Reads chapters_metadata.json and extracts selected clips

param(
    [string]$MetadataFile = "E:\GoPro Raw\Nolan - PWAA\chapters_metadata.json",
    [string]$OutputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips"
)

# ============================================================================
# FUNCTIONS
# ============================================================================

function Format-Seconds {
    param([double]$TotalSeconds)
    return [TimeSpan]::FromSeconds($TotalSeconds).ToString("hh\:mm\:ss\.fff")
}

# ============================================================================
# MAIN
# ============================================================================

Write-Host "=== GoPro Clip Extractor ===" -ForegroundColor Cyan
Write-Host ""

# Load metadata
if (!(Test-Path $MetadataFile)) {
    Write-Host "Metadata file not found: $MetadataFile" -ForegroundColor Red
    Write-Host "Run analyze_gopro_metadata.ps1 first to generate it."
    exit 1
}

$metadata = Get-Content $MetadataFile -Raw | ConvertFrom-Json
Write-Host "Loaded metadata from: $MetadataFile"
Write-Host "Generated: $($metadata.GeneratedAt)"
Write-Host "Total chapters: $($metadata.TotalChapters)"
Write-Host ""

# Create output folder
if (!(Test-Path $OutputFolder)) {
    New-Item -ItemType Directory -Path $OutputFolder | Out-Null
    Write-Host "Created output folder: $OutputFolder"
}

# Display all chapters chronologically
Write-Host "All Chapters (Chronological Order):" -ForegroundColor Cyan
Write-Host ("-" * 60)
Write-Host ("{0,3} {1,-12} {2,-15} {3,-8} {4,-15}" -f "#", "Period", "Clock Time", "Ch#", "Video Time")
Write-Host ("-" * 60)

$chapters = $metadata.AllChaptersChronological
for ($i = 0; $i -lt $chapters.Count; $i++) {
    $ch = $chapters[$i]
    Write-Host ("{0,3} {1,-12} {2,-15} {3,-8} {4,-15}" -f ($i + 1), $ch.Period, $ch.ClockTimeFormatted, "Ch$($ch.ChapterNumber)", $ch.VideoTimeFormatted)
}

# Selection prompt
Write-Host ""
Write-Host "Selection options:" -ForegroundColor Yellow
Write-Host "  - Enter a number (e.g., 5) for a single clip"
Write-Host "  - Enter a range (e.g., 1-10) for multiple clips"
Write-Host "  - Enter 'all' for all clips"
Write-Host "  - Enter comma-separated (e.g., 1,3,5,8) for specific clips"
Write-Host ""
$selection = Read-Host "Select clips to extract (default: all)"

# Parse selection
$selectedIndices = @()
if ($selection -eq 'all' -or $selection -eq 'a' -or $selection -eq '') {
    $selectedIndices = 0..($chapters.Count - 1)
} elseif ($selection -match "^(\d+)-(\d+)$") {
    $start = [int]$matches[1] - 1
    $end = [int]$matches[2] - 1
    $selectedIndices = $start..$end
} elseif ($selection -match ",") {
    $selectedIndices = $selection -split "," | ForEach-Object { [int]$_.Trim() - 1 }
} else {
    $selectedIndices = @([int]$selection - 1)
}

Write-Host ""
Write-Host "Selected $($selectedIndices.Count) clip(s) to extract."

# Timing settings
Write-Host ""
$beforeInput = Read-Host "Seconds before highlight (default: 8)"
$secondsBefore = if ($beforeInput -eq '') { 8 } else { [double]$beforeInput }

$afterInput = Read-Host "Seconds after highlight (default: 2)"
$secondsAfter = if ($afterInput -eq '') { 2 } else { [double]$afterInput }

$clipDuration = $secondsBefore + $secondsAfter
Write-Host "`nClip settings: ${secondsBefore}s before + ${secondsAfter}s after = ${clipDuration}s total" -ForegroundColor Cyan
Write-Host ""

# Extract clips
$extractedCount = 0
$errorCount = 0

foreach ($idx in $selectedIndices) {
    if ($idx -lt 0 -or $idx -ge $chapters.Count) {
        Write-Host "Skipping invalid index: $($idx + 1)" -ForegroundColor Yellow
        continue
    }

    $ch = $chapters[$idx]
    $globalNum = $idx + 1

    # Calculate clip boundaries
    $videoTimeSec = $ch.VideoTimeSec
    $clipStartSec = [Math]::Max(0, $videoTimeSec - $secondsBefore)

    # Two-pass seeking for accuracy
    $roughSeek = [Math]::Max(0, $clipStartSec - 60)
    $fineSeek = $clipStartSec - $roughSeek

    # Output filename: GlobalNum_ClockTime_Period_ChNum.mp4
    $clockSortable = $ch.ClockTimeSortable
    $periodShort = $ch.PeriodShort
    $chNum = $ch.ChapterNumber

    $outputFile = Join-Path $OutputFolder ("{0:D3}_{1}_{2}_Ch{3:D2}.mp4" -f $globalNum, $clockSortable, $periodShort, $chNum)

    Write-Host ("[{0}/{1}] {2} {3} Ch{4} -> {5}" -f $globalNum, $chapters.Count, $ch.Period, $ch.ClockTimeFormatted, $chNum, ($outputFile | Split-Path -Leaf))

    # Check video file exists
    if (!(Test-Path $ch.VideoFile)) {
        Write-Host "  ERROR: Video file not found: $($ch.VideoFile)" -ForegroundColor Red
        $errorCount++
        continue
    }

    # Extract clip
    ffmpeg -hide_banner -loglevel warning -y `
        -ss $roughSeek `
        -i $ch.VideoFile `
        -ss $fineSeek `
        -t $clipDuration `
        -c:v libx264 -preset slow -crf 18 `
        -c:a aac -b:a 192k `
        $outputFile

    if ($LASTEXITCODE -eq 0) {
        Write-Host "  Done!" -ForegroundColor Green
        $extractedCount++
    } else {
        Write-Host "  Error extracting clip" -ForegroundColor Red
        $errorCount++
    }
}

# Summary
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "COMPLETE" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Extracted: $extractedCount clips"
if ($errorCount -gt 0) {
    Write-Host "Errors: $errorCount" -ForegroundColor Red
}
Write-Host "Output folder: $OutputFolder"
Write-Host ""
Write-Host "Files are named with global order number + clock time for easy sorting."
