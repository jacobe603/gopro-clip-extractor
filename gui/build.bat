@echo off
set CGO_ENABLED=1
set PATH=C:\msys64\mingw64\bin;C:\Program Files\Go\bin;%PATH%
cd /d E:\Apps\gopro-clip-extractor\gui
echo Building gopro-gui.exe...
"C:\Program Files\Go\bin\go.exe" build -o gopro-gui.exe
if %ERRORLEVEL% EQU 0 (
    echo Build successful!
) else (
    echo Build failed with error code %ERRORLEVEL%
)
