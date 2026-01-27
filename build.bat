@echo off
cd /d G:\Apps\gopro-clip-extractor\gui
set CGO_ENABLED=1
set CC=C:\msys64\mingw64\bin\gcc.exe
set PATH=C:\msys64\mingw64\bin;%PATH%
go build -o ..\gopro-gui.exe .
if %ERRORLEVEL% EQU 0 (
    echo Build successful!
) else (
    echo Build failed with error %ERRORLEVEL%
)
