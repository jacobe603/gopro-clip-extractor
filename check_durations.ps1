$ffprobe = "C:\Users\jacob\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-8.0.1-full_build\bin\ffprobe.exe"
$folder = "D:\Video\Extract"

Get-ChildItem "$folder\*.mp4" | Where-Object { $_.Name -notmatch 'combined' } | ForEach-Object {
    $dur = & $ffprobe -v error -show_entries format=duration -of csv=p=0 $_.FullName 2>&1
    [PSCustomObject]@{
        Name = $_.Name
        Duration = [math]::Round([double]$dur, 1)
        SizeMB = [math]::Round($_.Length / 1MB, 1)
    }
} | Format-Table -AutoSize
