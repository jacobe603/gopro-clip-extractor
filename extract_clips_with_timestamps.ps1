# Extract clips based on chapter markers with real-world timestamps
# Maps chapter offsets to actual clock time from original GoPro timecodes
# Output files are named with real timestamps for chronological sorting

# ============================================================================
# CONFIGURATION - Map each period to its source GoPro file for timecode
# ============================================================================
$periods = @(
    @{
        Name = "1st Period"
        VideoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 1st Period.mp4"
        MetadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE1.txt"
        SourceGoPro = "E:\GoPro Raw\Nolan - PWAA\GX010060.MP4"  # Original file for timecode
        OutputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips1"
    },
    @{
        Name = "2nd Period"
        VideoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 2nd Period.mp4"
        MetadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE2.txt"
        SourceGoPro = "E:\GoPro Raw\Nolan - PWAA\GX010062.MP4"  # Adjust if different
        OutputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips2"
    },
    @{
        Name = "3rd Period"
        VideoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 3rd Period.mp4"
        MetadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE3.txt"
        SourceGoPro = "E:\GoPro Raw\Nolan - PWAA\GX010063.MP4"  # Adjust if different
        OutputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips3"
    }
)

# ============================================================================
# FUNCTIONS
# ============================================================================

function Get-GoProTimecode {
    param([string]$FilePath)

    $output = & ffprobe -v quiet -show_entries stream_tags=timecode -select_streams d:0 -of csv=p=0 $FilePath 2>$null
    if ($output -match "(\d{2}):(\d{2}):(\d{2}):(\d{2})") {
        $hours = [int]$matches[1]
        $minutes = [int]$matches[2]
        $seconds = [int]$matches[3]
        $frames = [int]$matches[4]
        # Convert to total seconds (assuming ~60fps, frames add fractional seconds)
        return ($hours * 3600) + ($minutes * 60) + $seconds + ($frames / 60.0)
    }
    return $null
}

function Convert-SecondsToTimecode {
    param([double]$TotalSeconds)

    $hours = [Math]::Floor($TotalSeconds / 3600)
    $minutes = [Math]::Floor(($TotalSeconds % 3600) / 60)
    $seconds = [Math]::Floor($TotalSeconds % 60)
    $ms = [Math]::Floor(($TotalSeconds % 1) * 1000)

    return "{0:D2}-{1:D2}-{2:D2}-{3:D3}" -f $hours, $minutes, $seconds, $ms
}

function Get-ChapterStarts {
    param([string]$MetadataFile)

    $chapters = @()
    $content = Get-Content $MetadataFile
    foreach ($line in $content) {
        if ($line -match "^START=(\d+)") {
            $chapters += [int]$matches[1]
        }
    }
    return $chapters
}

# ============================================================================
# MAIN SCRIPT
# ============================================================================

Write-Host "=== GoPro Clip Extractor with Real Timestamps ===" -ForegroundColor Cyan
Write-Host ""

# Display available periods
Write-Host "Available periods:"
for ($p = 0; $p -lt $periods.Count; $p++) {
    $period = $periods[$p]
    if (Test-Path $period.VideoFile) {
        Write-Host "  $($p + 1). $($period.Name)" -ForegroundColor Green
    } else {
        Write-Host "  $($p + 1). $($period.Name) [FILE NOT FOUND]" -ForegroundColor Red
    }
}

# Select period
Write-Host ""
$periodSelection = Read-Host "Select period (1-$($periods.Count)), or 'all' (default: all)"

if ($periodSelection -eq 'all' -or $periodSelection -eq 'a' -or $periodSelection -eq '') {
    $selectedPeriods = $periods
} else {
    $idx = [int]$periodSelection - 1
    if ($idx -lt 0 -or $idx -ge $periods.Count) {
        Write-Host "Invalid selection." -ForegroundColor Red
        exit 1
    }
    $selectedPeriods = @($periods[$idx])
}

# Timing settings
$beforeInput = Read-Host "Seconds before highlight (default: 8)"
$secondsBefore = if ($beforeInput -eq '') { 8 } else { [double]$beforeInput }

$afterInput = Read-Host "Seconds after highlight (default: 2)"
$secondsAfter = if ($afterInput -eq '') { 2 } else { [double]$afterInput }

$clipDuration = $secondsBefore + $secondsAfter
Write-Host "`nClip settings: ${secondsBefore}s before + ${secondsAfter}s after = ${clipDuration}s total" -ForegroundColor Cyan

# Process each selected period
foreach ($period in $selectedPeriods) {
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "Processing: $($period.Name)" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan

    # Validate files exist
    if (!(Test-Path $period.VideoFile)) {
        Write-Host "  Video file not found: $($period.VideoFile)" -ForegroundColor Red
        continue
    }
    if (!(Test-Path $period.MetadataFile)) {
        Write-Host "  Metadata file not found: $($period.MetadataFile)" -ForegroundColor Red
        continue
    }

    # Get starting timecode from original GoPro file
    $startingTimecodeSeconds = $null
    if (Test-Path $period.SourceGoPro) {
        $startingTimecodeSeconds = Get-GoProTimecode -FilePath $period.SourceGoPro
        if ($startingTimecodeSeconds) {
            $tcFormatted = [TimeSpan]::FromSeconds($startingTimecodeSeconds).ToString("hh\:mm\:ss")
            Write-Host "  Source timecode from $($period.SourceGoPro | Split-Path -Leaf): $tcFormatted" -ForegroundColor Green
        }
    }

    if ($null -eq $startingTimecodeSeconds) {
        Write-Host "  Warning: Could not get timecode from source. Using video offset only." -ForegroundColor Yellow
        $startingTimecodeSeconds = 0
    }

    # Create output folder
    if (!(Test-Path $period.OutputFolder)) {
        New-Item -ItemType Directory -Path $period.OutputFolder | Out-Null
    }

    # Parse chapters
    $chapterStarts = Get-ChapterStarts -MetadataFile $period.MetadataFile
    Write-Host "  Found $($chapterStarts.Count) chapters"

    # Display chapters with real timestamps
    Write-Host "`n  Chapters:" -ForegroundColor Yellow
    for ($i = 0; $i -lt $chapterStarts.Count; $i++) {
        $chapterNum = $i + 1
        $chapterTimeSec = $chapterStarts[$i] / 1000.0
        $realTimeSec = $startingTimecodeSeconds + $chapterTimeSec
        $realTimeFormatted = [TimeSpan]::FromSeconds($realTimeSec).ToString("hh\:mm\:ss\.fff")
        $videoTimeFormatted = [TimeSpan]::FromSeconds($chapterTimeSec).ToString("hh\:mm\:ss\.fff")
        Write-Host ("    {0,2}. Video: {1}  |  Clock: {2}" -f $chapterNum, $videoTimeFormatted, $realTimeFormatted)
    }

    # Ask which chapters to extract
    Write-Host ""
    $chapterSelection = Read-Host "  Enter chapter number, range (e.g., 1-5), or 'all' (default: all)"

    if ($chapterSelection -eq 'all' -or $chapterSelection -eq 'a' -or $chapterSelection -eq '') {
        $chaptersToProcess = 0..($chapterStarts.Count - 1)
    } elseif ($chapterSelection -match "^(\d+)-(\d+)$") {
        $startChap = [int]$matches[1] - 1
        $endChap = [int]$matches[2] - 1
        $chaptersToProcess = $startChap..$endChap
    } else {
        $chaptersToProcess = @([int]$chapterSelection - 1)
    }

    Write-Host ""

    # Extract each chapter
    foreach ($i in $chaptersToProcess) {
        if ($i -lt 0 -or $i -ge $chapterStarts.Count) {
            Write-Host "  Skipping invalid chapter index: $($i + 1)" -ForegroundColor Yellow
            continue
        }

        $chapterNum = $i + 1
        $startMs = $chapterStarts[$i]

        # Calculate times
        $chapterTimeSec = $startMs / 1000.0
        $clipStartSec = [Math]::Max(0, $chapterTimeSec - $secondsBefore)
        $realTimeSec = $startingTimecodeSeconds + $chapterTimeSec

        # Two-pass seeking
        $roughSeek = [Math]::Max(0, $clipStartSec - 60)
        $fineSeek = $clipStartSec - $roughSeek

        # Format timestamps for filename
        $realTimeFormatted = Convert-SecondsToTimecode -TotalSeconds $realTimeSec
        $videoTimeFormatted = [TimeSpan]::FromSeconds($chapterTimeSec).ToString("hh\:mm\:ss\.fff")

        # Filename: RealTime_Period_ChapterNum.mp4 (sorts chronologically)
        $periodShort = $period.Name -replace " ", ""
        $outputFile = Join-Path $period.OutputFolder ("${realTimeFormatted}_${periodShort}_Ch{0:D2}.mp4" -f $chapterNum)

        Write-Host "  Extracting Ch $chapterNum (Clock: $($realTimeFormatted -replace '-',':')) -> $($outputFile | Split-Path -Leaf)"

        # Extract clip
        ffmpeg -hide_banner -loglevel warning -y `
            -ss $roughSeek `
            -i $period.VideoFile `
            -ss $fineSeek `
            -t $clipDuration `
            -c:v libx264 -preset slow -crf 18 `
            -c:a aac -b:a 192k `
            $outputFile

        if ($LASTEXITCODE -eq 0) {
            Write-Host "    Done!" -ForegroundColor Green
        } else {
            Write-Host "    Error extracting clip" -ForegroundColor Red
        }
    }
}

Write-Host "`n=== Finished ===" -ForegroundColor Cyan
Write-Host "Clips are named with real timestamps (HH-MM-SS-mmm) for chronological sorting."
Write-Host "Sort by filename to see all clips in game order across periods."
