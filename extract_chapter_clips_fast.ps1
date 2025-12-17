# Extract clips based on chapter markers (optimized version)
# Configurable seconds before and after each chapter start
# Uses two-pass seeking for speed + accuracy

$videoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 3rd Period.mp4"
$metadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE3.txt"
$outputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips3"

# Create output folder if it doesn't exist
if (!(Test-Path $outputFolder)) {
    New-Item -ItemType Directory -Path $outputFolder | Out-Null
}

# Parse chapter start times from metadata file
$chapterStarts = @()
$content = Get-Content $metadataFile
foreach ($line in $content) {
    if ($line -match "^START=(\d+)") {
        $chapterStarts += [int]$matches[1]
    }
}

Write-Host "Found $($chapterStarts.Count) chapters:`n"

# Display all chapters with their timestamps
for ($i = 0; $i -lt $chapterStarts.Count; $i++) {
    $chapterNum = $i + 1
    $chapterTimeSec = $chapterStarts[$i] / 1000.0
    $chapterTimeFormatted = [TimeSpan]::FromSeconds($chapterTimeSec).ToString("hh\:mm\:ss\.fff")
    Write-Host ("  {0,2}. {1}" -f $chapterNum, $chapterTimeFormatted)
}

# Prompt user for chapter selection
Write-Host ""
$selection = Read-Host "Enter chapter number to extract, or 'all' (default: all)"

# Determine which chapters to process
if ($selection -eq 'all' -or $selection -eq 'a' -or $selection -eq '') {
    $chaptersToProcess = 0..($chapterStarts.Count - 1)
    Write-Host "  -> Extracting all $($chapterStarts.Count) chapters"
} else {
    $selectedNum = [int]$selection
    if ($selectedNum -lt 1 -or $selectedNum -gt $chapterStarts.Count) {
        Write-Host "Invalid chapter number. Must be between 1 and $($chapterStarts.Count)." -ForegroundColor Red
        exit 1
    }
    $chaptersToProcess = @($selectedNum - 1)  # Convert to 0-based index
    Write-Host "  -> Extracting chapter $selectedNum"
}

# Prompt for timing settings
$beforeInput = Read-Host "Seconds before highlight (default: 8)"
$secondsBefore = if ($beforeInput -eq '') { 8 } else { [double]$beforeInput }

$afterInput = Read-Host "Seconds after highlight (default: 2)"
$secondsAfter = if ($afterInput -eq '') { 2 } else { [double]$afterInput }

$clipDuration = $secondsBefore + $secondsAfter
Write-Host "`nClip settings: ${secondsBefore}s before + ${secondsAfter}s after = ${clipDuration}s total`n"

# Extract clips for selected chapter(s)
foreach ($i in $chaptersToProcess) {
    $chapterNum = $i + 1
    $startMs = $chapterStarts[$i]

    # Convert to seconds and calculate clip boundaries
    $chapterTimeSec = $startMs / 1000.0
    $clipStartSec = [Math]::Max(0, $chapterTimeSec - $secondsBefore)

    # Two-pass seeking: fast keyframe seek to get close, then accurate seek
    # Using 60 second buffer to handle varying keyframe intervals in GoPro footage
    $roughSeek = [Math]::Max(0, $clipStartSec - 60)
    $fineSeek = $clipStartSec - $roughSeek

    # Format for display
    $chapterTimeFormatted = [TimeSpan]::FromSeconds($chapterTimeSec).ToString("hh\:mm\:ss\.fff")

    $outputFile = Join-Path $outputFolder ("Chapter_{0:D2}_{1}.mp4" -f $chapterNum, ($chapterTimeFormatted -replace ":", "-" -replace "\.", "-"))

    Write-Host "Extracting Chapter $chapterNum (mark at $chapterTimeFormatted) -> $outputFile"

    # High quality encoding with CRF 18 (very high quality)
    ffmpeg -hide_banner -loglevel warning -y `
        -ss $roughSeek `
        -i $videoFile `
        -ss $fineSeek `
        -t $clipDuration `
        -c:v libx264 -preset slow -crf 18 `
        -c:a aac -b:a 192k `
        $outputFile

    if ($LASTEXITCODE -eq 0) {
        Write-Host "  Done!" -ForegroundColor Green
    } else {
        Write-Host "  Error extracting clip" -ForegroundColor Red
    }
}

Write-Host "`nFinished extracting $($chaptersToProcess.Count) clip(s) to: $outputFolder"
