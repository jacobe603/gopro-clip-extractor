$ffprobe = "C:\Users\jacob\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-8.0.1-full_build\bin\ffprobe.exe"

Write-Host "=== Original clip (1Period) ===" -ForegroundColor Green
& $ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,profile,level,bit_rate,width,height,pix_fmt -of default=noprint_wrappers=1 "D:\Video\Extract\001_13-37-45-231_1Period_Ch01.mp4" 2>&1

Write-Host "`n=== Flipped clip (3Period) ===" -ForegroundColor Yellow
& $ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,profile,level,bit_rate,width,height,pix_fmt -of default=noprint_wrappers=1 "D:\Video\Extract\029_14-38-06-604_3Period_Ch01.mp4" 2>&1
