$ffprobe = "C:\Users\jacob\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-8.0.1-full_build\bin\ffprobe.exe"
$folder = "D:\Video\Extract"

Get-ChildItem "$folder\*.mp4" | Where-Object { $_.Name -notmatch 'combined' } | ForEach-Object {
    $info = & $ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,width,height,pix_fmt -of csv=p=0 $_.FullName 2>&1
    $parts = $info -split ','
    [PSCustomObject]@{
        Name = $_.Name
        Codec = $parts[0]
        Resolution = "$($parts[1])x$($parts[2])"
        PixFmt = $parts[3]
    }
} | Format-Table -AutoSize
