# Analyze GoPro metadata - Extract chapters and map to real-world timestamps
# Outputs a JSON file with all chapter data for use by extract_clips.ps1

param(
    [string]$OutputFile = "E:\GoPro Raw\Nolan - PWAA\chapters_metadata.json"
)

# ============================================================================
# CONFIGURATION - Define your periods and source files
# ============================================================================
$periods = @(
    @{
        Name = "1st Period"
        VideoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 1st Period.mp4"
        MetadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE1.txt"
        SourceGoPro = "E:\GoPro Raw\Nolan - PWAA\GX010060.MP4"
    },
    @{
        Name = "2nd Period"
        VideoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 2nd Period.mp4"
        MetadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE2.txt"
        SourceGoPro = "E:\GoPro Raw\Nolan - PWAA\GX010062.MP4"
    },
    @{
        Name = "3rd Period"
        VideoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 3rd Period.mp4"
        MetadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE3.txt"
        SourceGoPro = "E:\GoPro Raw\Nolan - PWAA\GX010063.MP4"
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
        return @{
            Formatted = "$($matches[1]):$($matches[2]):$($matches[3])"
            TotalSeconds = ($hours * 3600) + ($minutes * 60) + $seconds + ($frames / 60.0)
        }
    }
    return $null
}

function Get-GoProCreationTime {
    param([string]$FilePath)

    $output = & ffprobe -v quiet -show_entries stream_tags=creation_time -select_streams v:0 -of csv=p=0 $FilePath 2>$null
    if ($output) {
        return $output.Trim()
    }
    return $null
}

function Get-VideoDuration {
    param([string]$FilePath)

    $output = & ffprobe -v quiet -show_entries format=duration -of csv=p=0 $FilePath 2>$null
    if ($output) {
        return [double]$output
    }
    return $null
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

function Format-Seconds {
    param([double]$TotalSeconds)
    return [TimeSpan]::FromSeconds($TotalSeconds).ToString("hh\:mm\:ss\.fff")
}

# ============================================================================
# MAIN
# ============================================================================

Write-Host "=== GoPro Metadata Analyzer ===" -ForegroundColor Cyan
Write-Host ""

# First, show available GoPro source files with their timecodes
Write-Host "Scanning original GoPro files for timecodes..." -ForegroundColor Yellow
Write-Host ""

$goProFiles = Get-ChildItem "E:\GoPro Raw\Nolan - PWAA\GX*.MP4" | Sort-Object Name
Write-Host "Original GoPro Files:" -ForegroundColor Cyan
Write-Host ("-" * 80)
Write-Host ("{0,-15} {1,-12} {2,-12} {3,-25}" -f "File", "Timecode", "Duration", "UTC Creation")
Write-Host ("-" * 80)

foreach ($file in $goProFiles) {
    $tc = Get-GoProTimecode -FilePath $file.FullName
    $dur = Get-VideoDuration -FilePath $file.FullName
    $creation = Get-GoProCreationTime -FilePath $file.FullName

    $tcStr = if ($tc) { $tc.Formatted } else { "N/A" }
    $durStr = if ($dur) { Format-Seconds -TotalSeconds $dur } else { "N/A" }
    $creationStr = if ($creation) { $creation.Substring(0, 19) } else { "N/A" }

    Write-Host ("{0,-15} {1,-12} {2,-12} {3,-25}" -f $file.Name, $tcStr, $durStr, $creationStr)
}
Write-Host ""

# Build chapter data
$allChapters = @()
$periodData = @()

foreach ($period in $periods) {
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Processing: $($period.Name)" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan

    # Check files exist
    $videoExists = Test-Path $period.VideoFile
    $metadataExists = Test-Path $period.MetadataFile
    $sourceExists = Test-Path $period.SourceGoPro

    if (!$videoExists) {
        Write-Host "  [SKIP] Video file not found" -ForegroundColor Red
        continue
    }
    if (!$metadataExists) {
        Write-Host "  [SKIP] Metadata file not found" -ForegroundColor Red
        continue
    }

    # Get source timecode
    $sourceTimecode = $null
    $startingSeconds = 0
    if ($sourceExists) {
        $sourceTimecode = Get-GoProTimecode -FilePath $period.SourceGoPro
        if ($sourceTimecode) {
            $startingSeconds = $sourceTimecode.TotalSeconds
            Write-Host "  Source: $($period.SourceGoPro | Split-Path -Leaf) -> $($sourceTimecode.Formatted)" -ForegroundColor Green
        }
    } else {
        Write-Host "  [WARN] Source GoPro not found, using offset only" -ForegroundColor Yellow
    }

    # Get video duration
    $videoDuration = Get-VideoDuration -FilePath $period.VideoFile
    Write-Host "  Video duration: $(Format-Seconds -TotalSeconds $videoDuration)"

    # Parse chapters
    $chapterStarts = Get-ChapterStarts -MetadataFile $period.MetadataFile
    Write-Host "  Chapters found: $($chapterStarts.Count)"
    Write-Host ""

    # Display chapter table
    Write-Host ("  {0,3} {1,-15} {2,-15} {3,-15}" -f "#", "Video Time", "Clock Time", "Clock (sortable)")
    Write-Host ("  " + ("-" * 55))

    $periodChapters = @()
    for ($i = 0; $i -lt $chapterStarts.Count; $i++) {
        $chapterNum = $i + 1
        $videoTimeSec = $chapterStarts[$i] / 1000.0
        $clockTimeSec = $startingSeconds + $videoTimeSec

        $videoTimeStr = Format-Seconds -TotalSeconds $videoTimeSec
        $clockTimeStr = Format-Seconds -TotalSeconds $clockTimeSec
        $clockSortable = [TimeSpan]::FromSeconds($clockTimeSec).ToString("hh\-mm\-ss\-fff")

        Write-Host ("  {0,3} {1,-15} {2,-15} {3,-15}" -f $chapterNum, $videoTimeStr, $clockTimeStr, $clockSortable)

        $chapterData = @{
            Period = $period.Name
            PeriodShort = $period.Name -replace " ", ""
            ChapterNumber = $chapterNum
            VideoTimeMs = $chapterStarts[$i]
            VideoTimeSec = $videoTimeSec
            VideoTimeFormatted = $videoTimeStr
            ClockTimeSec = $clockTimeSec
            ClockTimeFormatted = $clockTimeStr
            ClockTimeSortable = $clockSortable
            VideoFile = $period.VideoFile
        }

        $periodChapters += $chapterData
        $allChapters += $chapterData
    }

    $periodData += @{
        Name = $period.Name
        VideoFile = $period.VideoFile
        MetadataFile = $period.MetadataFile
        SourceGoPro = $period.SourceGoPro
        SourceTimecode = if ($sourceTimecode) { $sourceTimecode.Formatted } else { $null }
        SourceTimecodeSeconds = $startingSeconds
        VideoDuration = $videoDuration
        ChapterCount = $chapterStarts.Count
        Chapters = $periodChapters
    }

    Write-Host ""
}

# Sort all chapters by clock time
$allChaptersSorted = $allChapters | Sort-Object ClockTimeSec

# Summary
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "SUMMARY - All Chapters (Chronological)" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host ("{0,3} {1,-12} {2,-15} {3,-10}" -f "#", "Period", "Clock Time", "Ch#")
Write-Host ("-" * 45)

$globalNum = 1
foreach ($ch in $allChaptersSorted) {
    Write-Host ("{0,3} {1,-12} {2,-15} {3,-10}" -f $globalNum, $ch.Period, $ch.ClockTimeFormatted, "Ch$($ch.ChapterNumber)")
    $globalNum++
}

# Output JSON
$output = @{
    GeneratedAt = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    TotalChapters = $allChapters.Count
    Periods = $periodData
    AllChaptersChronological = $allChaptersSorted
}

$output | ConvertTo-Json -Depth 5 | Set-Content $OutputFile -Encoding UTF8

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Metadata saved to: $OutputFile" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Next step: Run extract_clips.ps1 to extract clips using this metadata."
