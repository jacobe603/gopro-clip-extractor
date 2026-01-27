# GoPro Clip Extractor

A Windows GUI application for extracting highlight clips from GoPro footage using HiLight chapter markers. Features smart auto-detection, real-world timestamp mapping, and YouTube-ready video export.

## Features

- **Smart folder detection** - Auto-detects MOV, MP4, and metadata files
- **Auto-period creation** - Automatically creates periods based on MOV files found
- **Split file detection** - Detects and combines split GoPro recordings (GX010092 + GX020092)
- **HiLight tag extraction** - Reads GoPro chapter markers (HiLight button presses)
- **Real-world timestamps** - Maps chapter markers to actual clock time for chronological sorting
- **Overlap detection** - Automatically merges overlapping highlights to avoid repeated video
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
- **Detects split GoPro recordings** (see below)
- Auto-creates periods based on MOV files found
- Shows progress during scanning

**Split GoPro File Detection:**

GoPro cameras automatically split long recordings into multiple files (e.g., when exceeding 4GB). The app detects these split files by their naming pattern:
- Pattern: `GX##XXXX.MP4` or `GH##XXXX.MP4` (also `.MOV`)
- Example: `GX010092.MP4` + `GX020092.MP4` are parts 1 and 2 of video 0092

When split files are detected:
1. The app shows a "Split GoPro Files" section
2. Select which groups to combine (pre-checked by default)
3. Click "Combine Selected" to merge into single files
4. Chapter markers (HiLights) are preserved and merged with correct timestamps
5. Output: `GX_combined_0092.MP4` (or `.MOV`)

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

**Automatic Overlap Detection:**

When highlights are close together, their clips would contain repeated video. For example, with 2s before and 8s after (10s total):
- Highlight 1 at 1:03 → clip from 1:01 to 1:11
- Highlight 2 at 1:07 → clip from 1:05 to 1:15
- Without merging: 6 seconds of video would repeat

The app automatically detects these overlaps and merges them into a single longer clip:
- Merged clip: 1:01 to 1:15 (14 seconds, covers both highlights)
- Status shows: "2 overlapping highlight groups detected (4 highlights merged into 2 clips)"
- Output filename: `150405_1Period_Ch03-04.mp4` (indicates merged range)

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

### Overlap Detection Algorithm
When extracting clips, the app detects overlapping highlights to avoid repeated video:

1. Groups chapters by period (can't merge across different video files)
2. Sorts chapters by video time within each period
3. Checks if each chapter's clip start overlaps with the previous group's end
4. Merges overlapping chapters into single `ClipGroup` with extended duration

**Overlap condition:** `next_chapter_time - before_padding < current_group_end_time`

**Future extension (Option B):** The code in `gui/metadata/overlap.go` is documented to support user choice between auto-merge (current) and manual merge with warnings. See `OverlapInfo` field and `CalculateRecommendedAfterTime()` function.

## Building from Source

### Quick Build (Recommended)

From the project root:
```cmd
build.bat
```

This script:
- Sets `CGO_ENABLED=1` (required for Fyne GUI framework)
- Configures MSYS2 MinGW GCC compiler path
- Builds `gopro-gui.exe` in the project root

### Manual Build

```powershell
cd gui
$env:CGO_ENABLED=1
$env:CC = "C:\msys64\mingw64\bin\gcc.exe"
$env:PATH = "C:\msys64\mingw64\bin;" + $env:PATH
go build -o ..\gopro-gui.exe .
```

### Prerequisites for Building

1. **Go 1.21+** - Install via `winget install GoLang.Go`
2. **MSYS2 with MinGW-w64** - Required for CGO compilation
   - Install MSYS2 from https://www.msys2.org/
   - Run in MSYS2: `pacman -S mingw-w64-x86_64-gcc`
3. **FFmpeg** - Place in `bin/` folder or install via `winget install Gyan.FFmpeg`

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

### FFmpeg Filter Complex for Video Combining

When combining clips or exporting full games, the app uses FFmpeg's `filter_complex` instead of the concat demuxer. This handles two common issues:

**1. Unknown streams in DNxHR MOV files:**
Shutter Encoder's DNxHR output includes metadata streams (timecode, subtitles) that the concat demuxer can't handle:
```
[concat] Could not find codec parameters for stream 3 (Unknown: none)
```

**2. Different resolutions between clips:**
Clips extracted from different source files may have slightly different resolutions, causing concat filter failures:
```
[Parsed_concat_0] Input link parameters (1831x1030) do not match output (1855x1043)
```

**Solution:** The app builds a filter_complex that:
- Explicitly selects only video `[N:v]` and audio `[N:a]` streams (ignores unknown streams)
- Scales all videos to 1920x1080 with letterboxing for consistent dimensions
- Concatenates the normalized streams

```
[0:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v0];
[1:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v1];
[v0][0:a][v1][1:a]concat=n=2:v=1:a=1[outv][outa]
```

### Overlap Detection Algorithm
When extracting clips, the app detects overlapping highlights to avoid repeated video:

1. Groups chapters by period (can't merge across different video files)
2. Sorts chapters by video time within each period
3. Checks if each chapter's clip start overlaps with the previous group's end
4. Merges overlapping chapters into single `ClipGroup` with extended duration

**Overlap condition:** `next_chapter_time - before_padding < current_group_end_time`

**Future extension (Option B):** The code in `gui/metadata/overlap.go` is documented to support user choice between auto-merge (current) and manual merge with warnings. See `OverlapInfo` field and `CalculateRecommendedAfterTime()` function.

## Changelog

### 2025-01-27
- **Fixed:** Step 4 Combine now handles clips with different resolutions by scaling to 1920x1080
- **Fixed:** Step 5 Export Full Game now works with DNxHR MOV files (handles unknown streams)
- **Added:** Split GoPro file detection and combining in Step 1

## License

MIT License
