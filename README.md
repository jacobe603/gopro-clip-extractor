# GoPro Chapter Clip Extractor

PowerShell scripts to extract highlight clips from GoPro footage using chapter markers, with real-world timestamp mapping for chronological sorting across multiple video files.

## Features

- Extracts timecode data from original GoPro files (GPMF telemetry)
- Maps chapter markers to real-world clock time
- Outputs clips named with timestamps for easy chronological sorting
- Two-pass seeking for accurate clip boundaries
- Configurable clip duration (seconds before/after highlight)

## Scripts

### `analyze_gopro_metadata.ps1`
Analyzes GoPro source files and chapter metadata. Run this first to:
- Display timecodes from original GoPro files
- Parse chapter markers from FFmpeg metadata files
- Map chapters to real clock times
- Output a JSON file for the extraction script

### `extract_clips.ps1`
Extracts clips using the analyzed metadata. Features:
- Chronological display of all chapters across periods
- Flexible selection: single, range (1-10), comma-separated (1,3,5), or all
- Configurable timing (default: 8 sec before, 2 sec after highlight)
- Files named with global order + clock time for sorting

### `combine_clips.ps1`
Concatenates all clips into a single highlight reel:
- Uses ffmpeg concat demuxer (fast, no re-encoding)
- Combines clips in filename order (chronological with our naming)
- Outputs to parent folder with timestamp

### `extract_chapter_clips.ps1` / `extract_chapter_clips_fast.ps1`
Standalone scripts for quick single-file extraction.

## Workflow

1. **Record** with GoPro using HiLight tags to mark highlights
2. **Extract chapters** from your GoPro files:
   ```
   ffmpeg -i GX010060.MP4 -f ffmetadata FFMETADATAFILE.txt
   ```
3. **Analyze metadata**:
   ```powershell
   .\analyze_gopro_metadata.ps1
   ```
4. **Extract clips**:
   ```powershell
   .\extract_clips.ps1
   ```
5. **Combine into highlight reel** (optional):
   ```powershell
   .\combine_clips.ps1 -InputFolder .\Clips
   ```

## Configuration

Edit the `$periods` array in `analyze_gopro_metadata.ps1` to match your files:

```powershell
$periods = @(
    @{
        Name = "1st Period"
        VideoFile = "path\to\merged_video.mp4"
        MetadataFile = "path\to\FFMETADATAFILE.txt"
        SourceGoPro = "path\to\original\GX010060.MP4"  # For timecode
    }
)
```

## Requirements

- PowerShell 5.1+
- FFmpeg and FFprobe in PATH

## Technical Notes

### Two-Pass Seeking
Early versions had issues with clips being cut short (0kb files or truncated clips). This was caused by ffmpeg's keyframe seeking when `-ss` is placed before `-i`.

**Solution**: Two-pass seeking with 60-second buffer:
```powershell
$roughSeek = [Math]::Max(0, $clipStartSec - 60)  # Fast keyframe seek
$fineSeek = $clipStartSec - $roughSeek           # Accurate frame seek

ffmpeg -ss $roughSeek -i $video -ss $fineSeek -t $duration ...
```

### GoPro Timecode
Original GoPro files (GX*.MP4) contain:
- **Timecode track** (`tmcd`): SMPTE timecode from camera's real-time clock
- **GPMF telemetry** (`gpmd`): GPS, accelerometer, gyroscope data

Merged/re-encoded files lose this data. The scripts read timecodes from original files to map chapter markers to real clock time.

### Chapter Extraction
GoPro HiLight tags are stored as chapter markers. Extract with:
```
ffmpeg -i GX010060.MP4 -f ffmetadata FFMETADATAFILE.txt
```

Chapters appear as `START=` timestamps in milliseconds.

## Output Naming

Clips are named for chronological sorting:
```
001_11-49-22-509_1stPeriod_Ch01.mp4
002_11-49-46-933_1stPeriod_Ch02.mp4
...
025_12-48-52-123_3rdPeriod_Ch05.mp4
```

Format: `{GlobalOrder}_{ClockTime}_{Period}_Ch{Number}.mp4`

## Future Improvements

- [ ] Make paths configurable via command-line parameters or config file
- [ ] Auto-detect GoPro source files and match to periods by timestamp
- [ ] Add transitions between clips in combined video
- [ ] Support for multiple cameras/angles
- [ ] GUI wrapper for non-technical users
