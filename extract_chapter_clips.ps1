# Extract clips based on chapter markers
# 7 seconds before and 2 seconds after each chapter start

$videoFile = "E:\GoPro Raw\Nolan - PWAA\Nolan - Fargo PWAA vs Moorhead PWAA 1st Period.mp4"
$metadataFile = "E:\GoPro Raw\Nolan - PWAA\FFMETADATAFILE1.txt"
$outputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips1"

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

Write-Host "Found $($chapterStarts.Count) chapters"

# Extract clips for each chapter
for ($i = 0; $i -lt $chapterStarts.Count; $i++) {
    $chapterNum = $i + 1
    $startMs = $chapterStarts[$i]
    
    # Convert to seconds and calculate clip boundaries
    $chapterTimeSec = $startMs / 1000.0
    $clipStartSec = [Math]::Max(0, $chapterTimeSec - 7)  # 7 seconds before
    $clipDuration = 9  # 7 before + 2 after = 9 seconds total
    
    # Format for display
    $chapterTimeFormatted = [TimeSpan]::FromSeconds($chapterTimeSec).ToString("hh\:mm\:ss\.fff")
    
    $outputFile = Join-Path $outputFolder ("Chapter_{0:D2}_{1}.mp4" -f $chapterNum, ($chapterTimeFormatted -replace ":", "-" -replace "\.", "-"))
    
    Write-Host "Extracting Chapter $chapterNum (mark at $chapterTimeFormatted) -> $outputFile"
    
    # High quality encoding with CRF 18 (very high quality)
    # Using -ss after -i for accurate frame-level seeking
    ffmpeg -hide_banner -loglevel warning -y `
        -i $videoFile `
        -ss $clipStartSec `
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

Write-Host "`nFinished extracting $($chapterStarts.Count) clips to: $outputFolder"
