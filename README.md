# GoPro Clip Extractor

A Windows GUI application for extracting highlight clips from GoPro footage using HiLight chapter markers. Features smart auto-detection, real-world timestamp mapping, and YouTube-ready video export.

## Features

- **Smart folder detection** - Auto-detects MOV, MP4, and metadata files
- **Auto-period creation** - Automatically creates periods based on MOV files found
- **HiLight tag extraction** - Reads GoPro chapter markers (HiLight button presses)
- **Real-world timestamps** - Maps chapter markers to actual clock time for chronological sorting
- **Flexible clip extraction** - Configurable seconds before/after each highlight
- **Two extraction modes** - Fast stream copy or re-encode with rotation/flip
- **Clip editing** - Adjust timing and re-extract individual clips
- **Combine clips** - Merge selected clips into a highlight reel
- **Full game export** - Combine period videos into single YouTube-ready file with chapter preservation

## Quick Start

### Option 1: Run Setup Script

```powershell
.\setup.ps1
```

This automatically installs Go, FFmpeg, and MSYS2/MinGW (required for building), then builds the app.

To auto-launch after build:
```powershell
.\setup.ps1 -RunAfterBuild
```

### Option 2: Run Pre-built Executable

If already built:
```powershell
.\gui\gopro-gui.exe
```

## Workflow Overview

```
Record with GoPro → Convert to CFR (Shutter Encoder) → Use GUI App
```

### Pre-requisite: Convert GoPro Videos

GoPro records in Variable Frame Rate (VFR), which causes audio sync issues. Convert to Constant Frame Rate (CFR) before using this app.

**Using [Shutter Encoder](https://www.shutterencoder.com/):**

1. Add your GoPro MP4 files
2. Select function: **DNxHR** (or ProRes for Mac)
3. Set quality: **HQ**
4. Recommended settings:
   - Scale: 1920x1080
   - Audio: Stereo
   - Color Grading → Exposure: +12 (for dark rink footage)
   - **Advanced Features → Preserve metadata** (keeps chapter markers!)
5. Click Start → Creates `.MOV` files

Keep matching filenames:
```
GX01.MP4 (original)  →  GX01.MOV (converted)
GX02.MP4 (original)  →  GX02.MOV (converted)
GX03.MP4 (original)  →  GX03.MOV (converted)
```

## GUI App - 5 Steps

### Step 1: Setup

Select your working folder containing the video files. The app automatically:

- Scans for MOV files (converted videos)
- Scans for MP4 files (original GoPro files)
- Scans for existing `_metadata.txt` files
- Checks each file for timecode and chapter markers
- Auto-creates periods based on MOV files found
- Shows progress during scanning

**Metadata detection:**
- If MOV has preserved chapters → Ready to use
- If `_metadata.txt` exists → Uses that
- If MP4 has chapters but MOV doesn't → Click "Extract Metadata" button

Click **Analyze & Continue** when all periods show ready status.

### Step 2: Extract Clips

- View all detected chapters across all periods in chronological order
- Select which chapters to extract (checkboxes)
- Configure timing: seconds before/after the highlight marker
- Choose extraction mode:
  - **Stream Copy (Fast)** - No re-encoding, preserves quality
  - **Re-encode** - Allows rotation, flipping, quality adjustment
- Extract clips with progress tracking

### Step 3: Edit Clips

- View extracted clips with thumbnails
- Adjust before/after timing for individual clips
- Re-extract individual clips with new timing
- Delete unwanted clips

### Step 4: Combine

- Select clips to combine into a highlight reel
- Drag to reorder (or sort by filename)
- Combine using stream copy (fast, no re-encoding)
- Preview total duration

### Step 5: Export Full Game

Combine all period MOV files into a single YouTube-ready video:

- Scans working folder for all MOV files
- Shows duration of each period and total
- Quality presets:
  - High Quality (CRF 18) ~12 Mbps
  - Balanced (CRF 20) ~8 Mbps
  - Smaller File (CRF 23) ~5 Mbps
- YouTube optimized: H.264 High profile, AAC audio, fast-start
- **Preserves chapter markers** with correct timestamp offsets
- Hardware acceleration (NVENC) with CPU fallback

## File Organization

For best results, keep all files in one folder:

```
Hockey_Game_2024-01-15/
├── GX01.MP4              # Original GoPro (1st period)
├── GX01.MOV              # Converted CFR (1st period)
├── GX01_metadata.txt     # Extracted chapters (auto-created if needed)
├── GX02.MP4              # Original GoPro (2nd period)
├── GX02.MOV              # Converted CFR (2nd period)
├── GX02_metadata.txt     # Extracted chapters
├── GX03.MP4              # Original GoPro (3rd period)
├── GX03.MOV              # Converted CFR (3rd period)
└── GX03_metadata.txt     # Extracted chapters
```

## Output Files

### Extracted Clips
Named for chronological sorting:
```
001_11-49-22-509_1Period_Ch01.mp4
002_11-49-46-933_1Period_Ch02.mp4
...
025_12-48-52-123_3Period_Ch05.mp4
```

Format: `{GlobalOrder}_{ClockTime}_{Period}_Ch{Number}.mp4`

### Combined Highlight Reel
```
Highlights_2024-01-15.mp4
```

### Full Game Export
```
FullGame_2024-01-15.mp4
```

## Requirements

- **Windows 10/11** with PowerShell 5.1+
- **FFmpeg** (installed automatically by setup.ps1)
- **[Shutter Encoder](https://www.shutterencoder.com/)** - for VFR to CFR conversion

### Build Requirements (if compiling from source)
- Go 1.21+
- MSYS2 with MinGW-w64 GCC (for CGO/Fyne)

## PowerShell Scripts

For advanced users, standalone PowerShell scripts are also available:

| Script | Purpose |
|--------|---------|
| `analyze_gopro_metadata.ps1` | Analyze GoPro files and chapter metadata |
| `extract_clips.ps1` | Extract clips with timestamp selection |
| `combine_clips.ps1` | Concatenate clips into highlight reel |
| `extract_chapter_clips.ps1` | Quick single-file extraction |

## Technical Notes

### GoPro Metadata
- **Timecode track** (`tmcd`): Real-time clock from camera
- **GPMF telemetry** (`gpmd`): GPS, accelerometer data
- **Chapter markers**: HiLight button presses stored as chapters

### TIMEBASE Handling
GoPro metadata uses `TIMEBASE=1/10000000`. The parser correctly handles this by dividing START values by the timebase denominator to get seconds.

### Two-Pass Seeking
For accurate clip extraction:
```
ffmpeg -ss [rough] -i video -ss [fine] -t duration ...
```
First seek is fast (keyframe), second is accurate (frame).

### Hardware Acceleration
- Uses NVIDIA NVENC when available
- Falls back to CPU (libx264) automatically

## Building from Source

```powershell
cd gui
.\build.bat
```

Or manually:
```powershell
$env:CGO_ENABLED=1
$env:PATH = "C:\msys64\mingw64\bin;" + $env:PATH
go build -o gopro-gui.exe
```

## License

MIT License
