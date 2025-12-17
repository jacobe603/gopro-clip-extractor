# Combine multiple clips into a single highlight reel
# Uses ffmpeg concat demuxer for fast, lossless concatenation

param(
    [string]$InputFolder = "E:\GoPro Raw\Nolan - PWAA\Clips",
    [string]$OutputFile = ""
)

Write-Host "=== Clip Combiner ===" -ForegroundColor Cyan
Write-Host ""

# Prompt for input folder if default doesn't exist
if (!(Test-Path $InputFolder)) {
    $InputFolder = Read-Host "Enter clips folder path"
}

if (!(Test-Path $InputFolder)) {
    Write-Host "Folder not found: $InputFolder" -ForegroundColor Red
    exit 1
}

# Find all mp4 files, sorted by name (chronological if using our naming convention)
$clips = Get-ChildItem -Path $InputFolder -Filter "*.mp4" | Sort-Object Name

if ($clips.Count -eq 0) {
    Write-Host "No .mp4 files found in: $InputFolder" -ForegroundColor Red
    exit 1
}

Write-Host "Found $($clips.Count) clips in: $InputFolder"
Write-Host ""

# Display clips
Write-Host "Clips to combine (in order):" -ForegroundColor Yellow
Write-Host ("-" * 60)
for ($i = 0; $i -lt $clips.Count; $i++) {
    Write-Host ("{0,3}. {1}" -f ($i + 1), $clips[$i].Name)
}
Write-Host ""

# Confirm
$confirm = Read-Host "Combine these $($clips.Count) clips? (Y/n)"
if ($confirm -eq 'n' -or $confirm -eq 'N') {
    Write-Host "Cancelled."
    exit 0
}

# Set output filename
if ($OutputFile -eq "") {
    $folderName = Split-Path $InputFolder -Leaf
    $timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm"
    $OutputFile = Join-Path (Split-Path $InputFolder -Parent) "${folderName}_combined_${timestamp}.mp4"
}

$outputNameInput = Read-Host "Output filename (default: $($OutputFile | Split-Path -Leaf))"
if ($outputNameInput -ne "") {
    if ($outputNameInput -notmatch "\.mp4$") {
        $outputNameInput += ".mp4"
    }
    $OutputFile = Join-Path (Split-Path $InputFolder -Parent) $outputNameInput
}

Write-Host ""
Write-Host "Output: $OutputFile"
Write-Host ""

# Create temporary file list for ffmpeg concat
$fileListPath = Join-Path $env:TEMP "ffmpeg_concat_list.txt"
$fileListContent = $clips | ForEach-Object { "file '$($_.FullName -replace "'","'\''")'" }
$fileListContent | Set-Content -Path $fileListPath -Encoding UTF8

Write-Host "Combining clips..." -ForegroundColor Cyan

# Run ffmpeg concat
# -c copy = no re-encoding (fast, lossless)
# -safe 0 = allow absolute paths
ffmpeg -hide_banner -loglevel warning -y `
    -f concat `
    -safe 0 `
    -i $fileListPath `
    -c copy `
    $OutputFile

if ($LASTEXITCODE -eq 0) {
    # Get output file size and duration
    $outputSize = (Get-Item $OutputFile).Length / 1MB
    $duration = & ffprobe -v quiet -show_entries format=duration -of csv=p=0 $OutputFile

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "SUCCESS!" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "Output: $OutputFile"
    Write-Host ("Size: {0:N1} MB" -f $outputSize)
    Write-Host ("Duration: {0}" -f [TimeSpan]::FromSeconds([double]$duration).ToString("hh\:mm\:ss"))
    Write-Host "Clips combined: $($clips.Count)"
} else {
    Write-Host ""
    Write-Host "Error combining clips" -ForegroundColor Red
    Write-Host "Check that all clips have the same codec settings."
}

# Clean up temp file
Remove-Item $fileListPath -ErrorAction SilentlyContinue
