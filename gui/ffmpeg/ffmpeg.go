package ffmpeg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// FFmpeg wraps ffmpeg and ffprobe executables
type FFmpeg struct {
	ffmpegPath  string
	ffprobePath string
	// currentCmd holds the currently running long operation (for cancellation)
	currentCmd *exec.Cmd
	// cancelFlag indicates if the current operation should be cancelled
	cancelFlag bool
}

// New creates a new FFmpeg wrapper, looking for binaries in the bin/ folder
func New() (*FFmpeg, error) {
	// Get the executable directory
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	// Try multiple locations for ffmpeg
	searchPaths := []string{
		filepath.Join(exeDir, "bin"),       // Next to executable
		filepath.Join(exeDir, "..", "bin"), // Parent/bin (for development)
		"bin",                              // Relative to working directory
	}

	var ffmpegPath, ffprobePath string
	ffmpegName := "ffmpeg"
	ffprobeName := "ffprobe"
	if runtime.GOOS == "windows" {
		ffmpegName = "ffmpeg.exe"
		ffprobeName = "ffprobe.exe"
	}

	for _, searchPath := range searchPaths {
		candidate := filepath.Join(searchPath, ffmpegName)
		if _, err := os.Stat(candidate); err == nil {
			ffmpegPath = candidate
			ffprobePath = filepath.Join(searchPath, ffprobeName)
			break
		}
	}

	// Fall back to PATH
	if ffmpegPath == "" {
		ffmpegPath, _ = exec.LookPath(ffmpegName)
		ffprobePath, _ = exec.LookPath(ffprobeName)
	}

	if ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg not found. Please place ffmpeg.exe in the bin/ folder")
	}
	if ffprobePath == "" {
		return nil, fmt.Errorf("ffprobe not found. Please place ffprobe.exe in the bin/ folder")
	}

	return &FFmpeg{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
	}, nil
}

// CancelExport cancels any currently running export operation
func (f *FFmpeg) CancelExport() error {
	f.cancelFlag = true
	if f.currentCmd != nil && f.currentCmd.Process != nil {
		return f.currentCmd.Process.Kill()
	}
	return nil
}

// IsCancelled returns true if the current operation was cancelled
func (f *FFmpeg) IsCancelled() bool {
	return f.cancelFlag
}

// ResetCancel resets the cancel flag for a new operation
func (f *FFmpeg) ResetCancel() {
	f.cancelFlag = false
	f.currentCmd = nil
}

// ExtractMetadata extracts chapter metadata from a video file using ffmpeg
func (f *FFmpeg) ExtractMetadata(inputPath, outputPath string) error {
	cmd := exec.Command(f.ffmpegPath,
		"-i", inputPath,
		"-f", "ffmetadata",
		"-y", // Overwrite output
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %s", stderr.String())
	}

	return nil
}

// GetTimecode extracts the timecode from a GoPro video file
// Returns the timecode as a string in format "HH:MM:SS:FF"
func (f *FFmpeg) GetTimecode(goProPath string) (string, error) {
	cmd := exec.Command(f.ffprobePath,
		"-v", "error",
		"-select_streams", "d:0",
		"-show_entries", "stream_tags=timecode",
		"-of", "csv=p=0",
		goProPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffprobe failed: %s", stderr.String())
	}

	timecode := strings.TrimSpace(stdout.String())
	if timecode == "" {
		return "", fmt.Errorf("no timecode found in %s", goProPath)
	}

	return timecode, nil
}

// GetDuration returns the duration of a video file in seconds
func (f *FFmpeg) GetDuration(videoPath string) (float64, error) {
	cmd := exec.Command(f.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		videoPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe failed: %s", stderr.String())
	}

	durationStr := strings.TrimSpace(stdout.String())
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// ExtractClip extracts a clip from a video file using two-pass seeking for accuracy
// Uses NVIDIA NVENC hardware encoding if available, falls back to CPU
func (f *FFmpeg) ExtractClip(inputPath, outputPath string, startSec, durationSec float64) error {
	// Two-pass seeking: rough seek to 60 seconds before, then fine seek
	// 60 seconds ensures we hit a keyframe before the target (GoPro has long GOP intervals)
	roughSeek := startSec - 60
	if roughSeek < 0 {
		roughSeek = 0
	}
	fineSeek := startSec - roughSeek

	// Try NVENC first (much faster with NVIDIA GPU)
	err := f.extractClipNVENC(inputPath, outputPath, roughSeek, fineSeek, durationSec)
	if err == nil {
		return nil
	}

	// Fall back to CPU encoding
	return f.extractClipCPU(inputPath, outputPath, roughSeek, fineSeek, durationSec)
}

// extractClipNVENC uses NVIDIA hardware encoding (YouTube-optimized settings)
func (f *FFmpeg) extractClipNVENC(inputPath, outputPath string, roughSeek, fineSeek, durationSec float64) error {
	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", roughSeek),
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", fineSeek),
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-c:v", "h264_nvenc",
		"-preset", "p4", // Good balance of speed/quality (p1=fastest, p7=slowest)
		"-profile:v", "high", // H.264 High profile for HD content
		"-rc", "constqp", // Constant quality mode
		"-qp", "18", // Quality level (similar to CRF 18)
		"-pix_fmt", "yuv420p", // Standard pixel format for compatibility
		"-c:a", "aac",
		"-ar", "48000", // 48kHz audio (YouTube recommended)
		"-b:a", "192k",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nvenc failed: %s", stderr.String())
	}

	return nil
}

// extractClipCPU uses software encoding (fallback, YouTube-optimized settings)
func (f *FFmpeg) extractClipCPU(inputPath, outputPath string, roughSeek, fineSeek, durationSec float64) error {
	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", roughSeek),
		"-i", inputPath,
		"-ss", fmt.Sprintf("%.3f", fineSeek),
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-c:v", "libx264",
		"-preset", "medium", // Good balance of speed/quality
		"-profile:v", "high", // H.264 High profile for HD content
		"-crf", "18",
		"-pix_fmt", "yuv420p", // Standard pixel format for compatibility
		"-c:a", "aac",
		"-ar", "48000", // 48kHz audio (YouTube recommended)
		"-b:a", "192k",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract failed: %s", stderr.String())
	}

	return nil
}

// ExtractClipStreamCopy extracts a clip without re-encoding (fast, keeps original codec)
func (f *FFmpeg) ExtractClipStreamCopy(inputPath, outputPath string, startSec, durationSec float64) error {
	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", inputPath,
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-c", "copy", // No re-encoding
		"-map", "0:v", // Only video
		"-map", "0:a", // Only audio
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg stream copy failed: %s", stderr.String())
	}

	return nil
}

// ClipChapter holds chapter information for embedding in extracted clips
type ClipChapter struct {
	OffsetMs int64  // Position in milliseconds from clip start
	Title    string // Chapter title
}

// ExtractClipWithChapters extracts a clip with embedded chapter markers
// Uses two-pass seeking for accuracy and embeds chapter metadata
func (f *FFmpeg) ExtractClipWithChapters(inputPath, outputPath string, startSec, durationSec float64, chapters []ClipChapter) error {
	// Two-pass seeking: rough seek to 60 seconds before, then fine seek
	roughSeek := startSec - 60
	if roughSeek < 0 {
		roughSeek = 0
	}
	fineSeek := startSec - roughSeek

	// Create metadata file with chapters
	metaFile, err := os.CreateTemp("", "ffmpeg-clip-meta-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metaFile.Name())

	fmt.Fprintf(metaFile, ";FFMETADATA1\n\n")

	durationMs := int64(durationSec * 1000)
	for i, ch := range chapters {
		// Calculate end time (next chapter start or clip end)
		var endMs int64
		if i < len(chapters)-1 {
			endMs = chapters[i+1].OffsetMs
		} else {
			endMs = durationMs
		}

		fmt.Fprintf(metaFile, "[CHAPTER]\n")
		fmt.Fprintf(metaFile, "TIMEBASE=1/1000\n")
		fmt.Fprintf(metaFile, "START=%d\n", ch.OffsetMs)
		fmt.Fprintf(metaFile, "END=%d\n", endMs)
		fmt.Fprintf(metaFile, "title=%s\n\n", ch.Title)
	}
	metaFile.Close()

	// Try NVENC first, fall back to CPU
	err = f.extractClipWithChaptersNVENC(inputPath, metaFile.Name(), outputPath, roughSeek, fineSeek, durationSec)
	if err == nil {
		return nil
	}

	return f.extractClipWithChaptersCPU(inputPath, metaFile.Name(), outputPath, roughSeek, fineSeek, durationSec)
}

func (f *FFmpeg) extractClipWithChaptersNVENC(inputPath, metaFile, outputPath string, roughSeek, fineSeek, durationSec float64) error {
	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", roughSeek),
		"-i", inputPath,
		"-i", metaFile,
		"-ss", fmt.Sprintf("%.3f", fineSeek),
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-map", "0:v",
		"-map", "0:a",
		"-map_metadata", "1",
		"-map_chapters", "1",
		"-c:v", "h264_nvenc",
		"-preset", "p4",
		"-profile:v", "high",
		"-rc", "constqp",
		"-qp", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nvenc failed: %s", stderr.String())
	}

	return nil
}

func (f *FFmpeg) extractClipWithChaptersCPU(inputPath, metaFile, outputPath string, roughSeek, fineSeek, durationSec float64) error {
	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", roughSeek),
		"-i", inputPath,
		"-i", metaFile,
		"-ss", fmt.Sprintf("%.3f", fineSeek),
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-map", "0:v",
		"-map", "0:a",
		"-map_metadata", "1",
		"-map_chapters", "1",
		"-c:v", "libx264",
		"-preset", "medium",
		"-profile:v", "high",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cpu extract failed: %s", stderr.String())
	}

	return nil
}

// ExtractClipStreamCopyWithChapters extracts a clip without re-encoding but with chapter markers
func (f *FFmpeg) ExtractClipStreamCopyWithChapters(inputPath, outputPath string, startSec, durationSec float64, chapters []ClipChapter) error {
	// Create metadata file with chapters
	metaFile, err := os.CreateTemp("", "ffmpeg-clip-meta-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metaFile.Name())

	fmt.Fprintf(metaFile, ";FFMETADATA1\n\n")

	durationMs := int64(durationSec * 1000)
	for i, ch := range chapters {
		var endMs int64
		if i < len(chapters)-1 {
			endMs = chapters[i+1].OffsetMs
		} else {
			endMs = durationMs
		}

		fmt.Fprintf(metaFile, "[CHAPTER]\n")
		fmt.Fprintf(metaFile, "TIMEBASE=1/1000\n")
		fmt.Fprintf(metaFile, "START=%d\n", ch.OffsetMs)
		fmt.Fprintf(metaFile, "END=%d\n", endMs)
		fmt.Fprintf(metaFile, "title=%s\n\n", ch.Title)
	}
	metaFile.Close()

	cmd := exec.Command(f.ffmpegPath,
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", inputPath,
		"-i", metaFile.Name(),
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-map", "0:v",
		"-map", "0:a",
		"-map_metadata", "1",
		"-map_chapters", "1",
		"-c", "copy",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg stream copy with chapters failed: %s", stderr.String())
	}

	return nil
}

// ConcatClips concatenates multiple clips into a single output file
// Preserves and merges chapter markers from all input clips
func (f *FFmpeg) ConcatClips(inputPaths []string, outputPath string) error {
	// Ensure output has .mp4 extension
	if !strings.HasSuffix(strings.ToLower(outputPath), ".mp4") {
		outputPath = outputPath + ".mp4"
	}

	// Step 1: Get durations and chapters from each input clip
	var durations []float64
	var allChapters []ChapterInfo
	var totalDuration float64

	for i, inputPath := range inputPaths {
		// Get duration
		dur, err := f.GetDuration(inputPath)
		if err != nil {
			// If we can't get duration, fall back to simple concat without chapters
			return f.concatClipsSimple(inputPaths, outputPath)
		}
		durations = append(durations, dur)

		// Get chapters from this clip
		chapters, _ := f.GetChapters(inputPath)

		// Calculate time offset for this file
		var offset float64
		for j := 0; j < i; j++ {
			offset += durations[j]
		}

		// If clip has no chapters, add one at the start to mark the clip boundary
		if len(chapters) == 0 {
			// Use filename as chapter title
			baseName := filepath.Base(inputPath)
			// Remove extension for cleaner title
			title := strings.TrimSuffix(baseName, filepath.Ext(baseName))
			allChapters = append(allChapters, ChapterInfo{
				StartMs: int64(offset * 1000),
				EndMs:   int64((offset + dur) * 1000),
				Title:   title,
			})
		} else {
			// Add existing chapters with offset
			for _, ch := range chapters {
				allChapters = append(allChapters, ChapterInfo{
					StartMs: ch.StartMs + int64(offset*1000),
					EndMs:   ch.EndMs + int64(offset*1000),
					Title:   ch.Title,
				})
			}
		}

		totalDuration += dur
	}

	// Step 2: Create concat file list
	concatFile, err := os.CreateTemp("", "ffmpeg-concat-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create concat file: %w", err)
	}
	defer os.Remove(concatFile.Name())

	for _, path := range inputPaths {
		escapedPath := strings.ReplaceAll(path, "\\", "/")
		escapedPath = strings.ReplaceAll(escapedPath, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escapedPath)
	}
	concatFile.Close()

	// Step 3: Create metadata file with merged chapters
	metaFile, err := os.CreateTemp("", "ffmpeg-meta-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metaFile.Name())

	fmt.Fprintf(metaFile, ";FFMETADATA1\n")
	fmt.Fprintf(metaFile, "title=Combined Clips\n")
	fmt.Fprintf(metaFile, "\n")

	for _, ch := range allChapters {
		fmt.Fprintf(metaFile, "[CHAPTER]\n")
		fmt.Fprintf(metaFile, "TIMEBASE=1/1000\n")
		fmt.Fprintf(metaFile, "START=%d\n", ch.StartMs)
		fmt.Fprintf(metaFile, "END=%d\n", ch.EndMs)
		if ch.Title != "" {
			fmt.Fprintf(metaFile, "title=%s\n", ch.Title)
		}
		fmt.Fprintf(metaFile, "\n")
	}
	metaFile.Close()

	// Step 4: Run ffmpeg with merged metadata
	// Note: DNxHR MOV files from Shutter Encoder have unknown metadata streams (stream 3+)
	// -err_detect ignore_err tells ffmpeg to continue despite probe warnings
	// We use explicit stream mapping to only copy video and audio streams
	cmd := exec.Command(f.ffmpegPath,
		"-err_detect", "ignore_err",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile.Name(),
		"-i", metaFile.Name(),
		"-map", "0:v:0", // First video stream only
		"-map", "0:a:0", // First audio stream only
		"-map_metadata", "1",
		"-map_chapters", "1",
		"-c", "copy",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat failed: %s", stderr.String())
	}

	return nil
}

// concatClipsSimple is a fallback that concatenates without chapter preservation
func (f *FFmpeg) concatClipsSimple(inputPaths []string, outputPath string) error {
	tempFile, err := os.CreateTemp("", "ffmpeg-concat-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	for _, path := range inputPaths {
		escapedPath := strings.ReplaceAll(path, "\\", "/")
		escapedPath = strings.ReplaceAll(escapedPath, "'", "'\\''")
		fmt.Fprintf(tempFile, "file '%s'\n", escapedPath)
	}
	tempFile.Close()

	cmd := exec.Command(f.ffmpegPath,
		"-err_detect", "ignore_err",
		"-f", "concat",
		"-safe", "0",
		"-i", tempFile.Name(),
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c", "copy",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat failed: %s", stderr.String())
	}

	return nil
}

// ConcatClipsWithEncode combines clips with re-encoding for smaller file size
// Preserves and merges chapter markers from all input clips
// If forceCPU is true, uses libx264 instead of NVENC
func (f *FFmpeg) ConcatClipsWithEncode(inputPaths []string, outputPath string, crf string, forceCPU bool) error {
	// Ensure output has .mp4 extension
	if !strings.HasSuffix(strings.ToLower(outputPath), ".mp4") {
		outputPath = outputPath + ".mp4"
	}

	// Step 1: Get durations and chapters from each input clip
	var durations []float64
	var allChapters []ChapterInfo

	for i, inputPath := range inputPaths {
		// Get duration
		dur, err := f.GetDuration(inputPath)
		if err != nil {
			dur = 0 // Continue without duration
		}
		durations = append(durations, dur)

		// Get chapters from this clip
		chapters, _ := f.GetChapters(inputPath)

		// Calculate time offset for this file
		var offset float64
		for j := 0; j < i; j++ {
			offset += durations[j]
		}

		// If clip has no chapters, add one at the start to mark the clip boundary
		if len(chapters) == 0 {
			title := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
			allChapters = append(allChapters, ChapterInfo{
				StartMs: int64(offset * 1000),
				EndMs:   int64((offset + dur) * 1000),
				Title:   title,
			})
		} else {
			// Add existing chapters with offset
			for _, ch := range chapters {
				allChapters = append(allChapters, ChapterInfo{
					StartMs: ch.StartMs + int64(offset*1000),
					EndMs:   ch.EndMs + int64(offset*1000),
					Title:   ch.Title,
				})
			}
		}
	}

	// Step 2: Create metadata file with merged chapters
	metaFile, err := os.CreateTemp("", "ffmpeg-meta-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metaFile.Name())

	fmt.Fprintf(metaFile, ";FFMETADATA1\n")
	fmt.Fprintf(metaFile, "title=Combined Clips\n")
	fmt.Fprintf(metaFile, "\n")

	for _, ch := range allChapters {
		fmt.Fprintf(metaFile, "[CHAPTER]\n")
		fmt.Fprintf(metaFile, "TIMEBASE=1/1000\n")
		fmt.Fprintf(metaFile, "START=%d\n", ch.StartMs)
		fmt.Fprintf(metaFile, "END=%d\n", ch.EndMs)
		if ch.Title != "" {
			fmt.Fprintf(metaFile, "title=%s\n", ch.Title)
		}
		fmt.Fprintf(metaFile, "\n")
	}
	metaFile.Close()

	// Step 3: Run ffmpeg with re-encoding using filter_complex concat
	// This avoids issues with unknown streams in DNxHR MOV files
	if forceCPU {
		return f.concatClipsEncodeCPU(inputPaths, metaFile.Name(), outputPath, crf)
	}

	// Try NVENC first, fall back to CPU
	err = f.concatClipsEncodeNVENC(inputPaths, metaFile.Name(), outputPath, crf)
	if err != nil {
		return f.concatClipsEncodeCPU(inputPaths, metaFile.Name(), outputPath, crf)
	}
	return nil
}

func (f *FFmpeg) concatClipsEncodeNVENC(inputPaths []string, metaFile, outputPath, crf string) error {
	qp := crf

	// Build ffmpeg command using filter_complex concat instead of concat demuxer
	// This avoids issues with unknown streams in DNxHR MOV files
	args := []string{}

	// Add each input file
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}

	// Add metadata file as last input
	args = append(args, "-i", metaFile)

	// Build filter_complex string: scale each video to 1920x1080, then concat
	// This handles clips with different resolutions (common when extracted from different source files)
	// Example: [0:v]scale=1920:1080:...[v0];[1:v]scale=1920:1080:...[v1];[v0][0:a][v1][1:a]concat=n=2:v=1:a=1[outv][outa]
	filterStr := ""
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d];", i, i)
	}
	// Add scaled video and audio streams to concat
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[v%d][%d:a]", i, i)
	}
	filterStr += fmt.Sprintf("concat=n=%d:v=1:a=1[outv][outa]", len(inputPaths))

	args = append(args,
		"-filter_complex", filterStr,
		"-map", "[outv]",
		"-map", "[outa]",
		"-map_metadata", fmt.Sprintf("%d", len(inputPaths)), // metadata file is last input
		"-map_chapters", fmt.Sprintf("%d", len(inputPaths)),
		"-c:v", "h264_nvenc",
		"-preset", "p4",
		"-profile:v", "high",
		"-rc", "constqp",
		"-qp", qp,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	f.currentCmd = cmd

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if f.cancelFlag {
			return fmt.Errorf("combine cancelled")
		}
		return fmt.Errorf("nvenc encode failed: %s", stderr.String())
	}

	return nil
}

func (f *FFmpeg) concatClipsEncodeCPU(inputPaths []string, metaFile, outputPath, crf string) error {
	// Build ffmpeg command using filter_complex concat instead of concat demuxer
	// This avoids issues with unknown streams in DNxHR MOV files
	args := []string{}

	// Add each input file
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}

	// Add metadata file as last input
	args = append(args, "-i", metaFile)

	// Build filter_complex string: scale each video to 1920x1080, then concat
	// This handles clips with different resolutions (common when extracted from different source files)
	filterStr := ""
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d];", i, i)
	}
	// Add scaled video and audio streams to concat
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[v%d][%d:a]", i, i)
	}
	filterStr += fmt.Sprintf("concat=n=%d:v=1:a=1[outv][outa]", len(inputPaths))

	args = append(args,
		"-filter_complex", filterStr,
		"-map", "[outv]",
		"-map", "[outa]",
		"-map_metadata", fmt.Sprintf("%d", len(inputPaths)), // metadata file is last input
		"-map_chapters", fmt.Sprintf("%d", len(inputPaths)),
		"-c:v", "libx264",
		"-preset", "medium",
		"-profile:v", "high",
		"-crf", crf,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	f.currentCmd = cmd

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if f.cancelFlag {
			return fmt.Errorf("combine cancelled")
		}
		return fmt.Errorf("cpu encode failed: %s", stderr.String())
	}

	return nil
}

// VideoMetadataInfo holds metadata information about a video file
type VideoMetadataInfo struct {
	HasTimecode  bool
	Timecode     string
	ChapterCount int
	HasChapters  bool
	CreationTime string
	Duration     float64
}

// CheckVideoMetadata checks if a video file has preserved metadata (timecode, chapters)
func (f *FFmpeg) CheckVideoMetadata(videoPath string) (*VideoMetadataInfo, error) {
	info := &VideoMetadataInfo{}

	// Get timecode from video stream tags
	timecode, err := f.GetTimecodeFromVideo(videoPath)
	if err == nil && timecode != "" {
		info.HasTimecode = true
		info.Timecode = timecode
	}

	// Get chapter count
	chapters, err := f.GetChapterCount(videoPath)
	if err == nil && chapters > 0 {
		info.HasChapters = true
		info.ChapterCount = chapters
	}

	// Get duration
	duration, err := f.GetDuration(videoPath)
	if err == nil {
		info.Duration = duration
	}

	return info, nil
}

// GetTimecodeFromVideo extracts timecode from video stream tags (works with converted MOV files)
func (f *FFmpeg) GetTimecodeFromVideo(videoPath string) (string, error) {
	// First try video stream tags
	cmd := exec.Command(f.ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream_tags=timecode",
		"-of", "csv=p=0",
		videoPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		timecode := strings.TrimSpace(stdout.String())
		if timecode != "" {
			return timecode, nil
		}
	}

	// Fall back to format tags (some files store it there)
	cmd = exec.Command(f.ffprobePath,
		"-v", "error",
		"-show_entries", "format_tags=timecode",
		"-of", "csv=p=0",
		videoPath,
	)

	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		timecode := strings.TrimSpace(stdout.String())
		if timecode != "" {
			return timecode, nil
		}
	}

	// Fall back to data stream (original GoPro files)
	return f.GetTimecode(videoPath)
}

// GetChapterCount returns the number of chapters in a video file
func (f *FFmpeg) GetChapterCount(videoPath string) (int, error) {
	cmd := exec.Command(f.ffprobePath,
		"-v", "error",
		"-show_chapters",
		"-of", "csv=p=0",
		videoPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe failed: %s", stderr.String())
	}

	// Count non-empty lines (each chapter produces output)
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	return count, nil
}

// ParseTimecode parses a timecode string "HH:MM:SS:FF" into total seconds
// Assumes ~60fps (GoPro standard)
func ParseTimecode(timecode string) (float64, error) {
	// Match HH:MM:SS:FF or HH:MM:SS;FF
	re := regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[:;](\d{2})`)
	matches := re.FindStringSubmatch(timecode)
	if matches == nil {
		return 0, fmt.Errorf("invalid timecode format: %s", timecode)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	frames, _ := strconv.Atoi(matches[4])

	// GoPro typically uses ~60fps
	const fps = 60.0
	totalSeconds := float64(hours*3600+minutes*60+seconds) + float64(frames)/fps

	return totalSeconds, nil
}

// ChapterInfo holds chapter timing information
type ChapterInfo struct {
	StartMs int64
	EndMs   int64
	Title   string
}

// GetChapters extracts chapter information from a video file
func (f *FFmpeg) GetChapters(videoPath string) ([]ChapterInfo, error) {
	cmd := exec.Command(f.ffprobePath,
		"-v", "error",
		"-show_chapters",
		"-print_format", "csv",
		videoPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %s", stderr.String())
	}

	var chapters []ChapterInfo
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "chapter,") {
			continue
		}
		// Format: chapter,id,time_base,start,start_time,end,end_time,title
		parts := strings.Split(line, ",")
		if len(parts) >= 7 {
			// start_time and end_time are in seconds as floats
			startTime, _ := strconv.ParseFloat(parts[4], 64)
			endTime, _ := strconv.ParseFloat(parts[6], 64)
			title := ""
			if len(parts) >= 8 {
				title = parts[7]
			}
			chapters = append(chapters, ChapterInfo{
				StartMs: int64(startTime * 1000),
				EndMs:   int64(endTime * 1000),
				Title:   title,
			})
		}
	}

	return chapters, nil
}

// VideoResolution holds video dimensions
type VideoResolution struct {
	Width  int
	Height int
}

// GetResolution returns the resolution of a video file
func (f *FFmpeg) GetResolution(videoPath string) (*VideoResolution, error) {
	cmd := exec.Command(f.ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		videoPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %s", stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	parts := strings.Split(output, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected resolution format: %s", output)
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse width: %w", err)
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse height: %w", err)
	}

	return &VideoResolution{Width: width, Height: height}, nil
}

// CombineSplitGoPro combines split GoPro files into a single file
// Uses stream copy if all files have matching dimensions, otherwise re-encodes
// Preserves and merges chapter markers (HiLights) from all input files
func (f *FFmpeg) CombineSplitGoPro(inputPaths []string, outputPath string) error {
	if len(inputPaths) < 2 {
		return fmt.Errorf("need at least 2 files to combine")
	}

	// Step 1: Check if all files have the same resolution
	var resolutions []*VideoResolution
	needsReencode := false
	var targetWidth, targetHeight int

	for _, inputPath := range inputPaths {
		res, err := f.GetResolution(inputPath)
		if err != nil {
			// If we can't get resolution, assume we need re-encoding
			needsReencode = true
			break
		}
		resolutions = append(resolutions, res)
	}

	if !needsReencode && len(resolutions) > 0 {
		targetWidth = resolutions[0].Width
		targetHeight = resolutions[0].Height
		for _, res := range resolutions[1:] {
			if res.Width != targetWidth || res.Height != targetHeight {
				needsReencode = true
				// Use the largest resolution as target
				if res.Width*res.Height > targetWidth*targetHeight {
					targetWidth = res.Width
					targetHeight = res.Height
				}
				break
			}
		}
	}

	// Step 2: Get durations and chapters from each file
	var durations []float64
	var allChapters []ChapterInfo

	for i, inputPath := range inputPaths {
		// Get duration
		dur, err := f.GetDuration(inputPath)
		if err != nil {
			return fmt.Errorf("failed to get duration of %s: %w", inputPath, err)
		}
		durations = append(durations, dur)

		// Get chapters (HiLights)
		chapters, _ := f.GetChapters(inputPath)

		// Calculate time offset for this file
		var offset float64
		for j := 0; j < i; j++ {
			offset += durations[j]
		}

		// Add chapters with adjusted timestamps
		for _, ch := range chapters {
			allChapters = append(allChapters, ChapterInfo{
				StartMs: ch.StartMs + int64(offset*1000),
				EndMs:   ch.EndMs + int64(offset*1000),
				Title:   ch.Title,
			})
		}
	}

	// Step 3: Create metadata file with merged chapters
	metaFile, err := os.CreateTemp("", "ffmpeg-meta-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metaFile.Name())

	fmt.Fprintf(metaFile, ";FFMETADATA1\n")
	fmt.Fprintf(metaFile, "\n")

	for _, ch := range allChapters {
		fmt.Fprintf(metaFile, "[CHAPTER]\n")
		fmt.Fprintf(metaFile, "TIMEBASE=1/1000\n")
		fmt.Fprintf(metaFile, "START=%d\n", ch.StartMs)
		fmt.Fprintf(metaFile, "END=%d\n", ch.EndMs)
		if ch.Title != "" {
			fmt.Fprintf(metaFile, "title=%s\n", ch.Title)
		}
		fmt.Fprintf(metaFile, "\n")
	}
	metaFile.Close()

	if needsReencode {
		// Use filter_complex with scale to normalize resolutions
		return f.combineSplitGoProReencode(inputPaths, metaFile.Name(), outputPath, targetWidth, targetHeight)
	}

	// Step 4: Stream copy (fast path for matching dimensions)
	concatFile, err := os.CreateTemp("", "ffmpeg-concat-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create concat file: %w", err)
	}
	defer os.Remove(concatFile.Name())

	for _, path := range inputPaths {
		escapedPath := strings.ReplaceAll(path, "\\", "/")
		escapedPath = strings.ReplaceAll(escapedPath, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escapedPath)
	}
	concatFile.Close()

	cmd := exec.Command(f.ffmpegPath,
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile.Name(),
		"-i", metaFile.Name(),
		"-map", "0:v",
		"-map", "0:a",
		"-map_metadata", "1",
		"-map_chapters", "1",
		"-c", "copy", // No re-encoding
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg combine failed: %s", stderr.String())
	}

	return nil
}

// combineSplitGoProReencode combines files with different resolutions using re-encoding
func (f *FFmpeg) combineSplitGoProReencode(inputPaths []string, metaFile, outputPath string, targetWidth, targetHeight int) error {
	// Build ffmpeg command with filter_complex to scale and concat
	args := []string{}

	// Add each input file
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}

	// Add metadata file as last input
	args = append(args, "-i", metaFile)

	// Build filter_complex: scale each input to target resolution, then concat
	// Example: [0:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2[v0];...
	filterStr := ""
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[%d:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d];",
			i, targetWidth, targetHeight, targetWidth, targetHeight, i)
	}
	// Add audio streams
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[v%d][%d:a]", i, i)
	}
	filterStr += fmt.Sprintf("concat=n=%d:v=1:a=1[outv][outa]", len(inputPaths))

	args = append(args,
		"-filter_complex", filterStr,
		"-map", "[outv]",
		"-map", "[outa]",
		"-map_metadata", fmt.Sprintf("%d", len(inputPaths)),
		"-map_chapters", fmt.Sprintf("%d", len(inputPaths)),
	)

	// Try NVENC first, fall back to CPU
	nvencArgs := append(args,
		"-c:v", "h264_nvenc",
		"-preset", "p4",
		"-profile:v", "high",
		"-rc", "constqp",
		"-qp", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-y",
		outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, nvencArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fall back to CPU encoding
	cpuArgs := append(args,
		"-c:v", "libx264",
		"-preset", "medium",
		"-profile:v", "high",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-y",
		outputPath,
	)

	cmd = exec.Command(f.ffmpegPath, cpuArgs...)
	stderr.Reset()
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg combine with re-encode failed: %s", stderr.String())
	}

	return nil
}

// ExportFullGame combines multiple MOV files into a single YouTube-ready video
// with re-encoding and merged chapter markers
// If forceCPU is true, uses libx264 instead of NVENC for better compression efficiency
func (f *FFmpeg) ExportFullGame(inputPaths []string, outputPath string, crf string, forceCPU bool, progress func(float64, string)) error {
	if len(inputPaths) == 0 {
		return fmt.Errorf("no input files")
	}

	// Step 1: Get durations and chapters from each file
	progress(0.05, "Analyzing source files...")

	var durations []float64
	var allChapters []ChapterInfo
	var totalDuration float64
	periodNames := []string{"1st Period", "2nd Period", "3rd Period", "OT"}

	for i, inputPath := range inputPaths {
		// Get duration
		dur, err := f.GetDuration(inputPath)
		if err != nil {
			return fmt.Errorf("failed to get duration of %s: %w", inputPath, err)
		}
		durations = append(durations, dur)

		// Get chapters
		chapters, _ := f.GetChapters(inputPath)

		// Calculate time offset for this file
		var offset float64
		for j := 0; j < i; j++ {
			offset += durations[j]
		}

		// Add period start marker
		periodName := "Period"
		if i < len(periodNames) {
			periodName = periodNames[i]
		} else {
			periodName = fmt.Sprintf("Period %d", i+1)
		}

		allChapters = append(allChapters, ChapterInfo{
			StartMs: int64(offset * 1000),
			EndMs:   int64((offset + dur) * 1000),
			Title:   periodName,
		})

		// Add HiLight chapters with offset
		for j, ch := range chapters {
			allChapters = append(allChapters, ChapterInfo{
				StartMs: ch.StartMs + int64(offset*1000),
				EndMs:   ch.EndMs + int64(offset*1000),
				Title:   fmt.Sprintf("%s - Highlight %d", periodName, j+1),
			})
		}

		totalDuration += dur
	}

	// Step 2: Create metadata file with chapters
	progress(0.1, "Preparing files...")
	metaFile, err := os.CreateTemp("", "ffmpeg-meta-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer os.Remove(metaFile.Name())

	fmt.Fprintf(metaFile, ";FFMETADATA1\n")
	fmt.Fprintf(metaFile, "title=Full Game\n")
	fmt.Fprintf(metaFile, "\n")

	for _, ch := range allChapters {
		fmt.Fprintf(metaFile, "[CHAPTER]\n")
		fmt.Fprintf(metaFile, "TIMEBASE=1/1000\n")
		fmt.Fprintf(metaFile, "START=%d\n", ch.StartMs)
		fmt.Fprintf(metaFile, "END=%d\n", ch.EndMs)
		fmt.Fprintf(metaFile, "title=%s\n", ch.Title)
		fmt.Fprintf(metaFile, "\n")
	}
	metaFile.Close()

	// Step 4: Run ffmpeg with YouTube-optimized settings using filter_complex
	// This handles DNxHR MOV files with unknown streams and different resolutions
	progress(0.15, "Encoding video (this may take a while)...")

	if forceCPU {
		progress(0.15, "Using CPU encoding for best compression...")
		err = f.exportFullGameCPU(inputPaths, metaFile.Name(), outputPath, crf)
	} else {
		err = f.exportFullGameNVENC(inputPaths, metaFile.Name(), outputPath, crf)
		if err != nil {
			progress(0.15, "GPU encoding not available, using CPU...")
			err = f.exportFullGameCPU(inputPaths, metaFile.Name(), outputPath, crf)
		}
	}

	if err != nil {
		return err
	}

	progress(1.0, "Export complete!")
	return nil
}

// exportFullGameNVENC exports using NVIDIA hardware encoding with filter_complex
func (f *FFmpeg) exportFullGameNVENC(inputPaths []string, metaFile, outputPath, crf string) error {
	qp := crf

	args := []string{}

	// Add each input file
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}

	// Add metadata file as last input
	args = append(args, "-i", metaFile)

	// Build filter_complex: scale each video to 1920x1080, then concat
	// This handles DNxHR MOV files with unknown streams and different resolutions
	filterStr := ""
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d];", i, i)
	}
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[v%d][%d:a]", i, i)
	}
	filterStr += fmt.Sprintf("concat=n=%d:v=1:a=1[outv][outa]", len(inputPaths))

	args = append(args,
		"-filter_complex", filterStr,
		"-map", "[outv]",
		"-map", "[outa]",
		"-map_metadata", fmt.Sprintf("%d", len(inputPaths)),
		"-map_chapters", fmt.Sprintf("%d", len(inputPaths)),
		"-c:v", "h264_nvenc",
		"-preset", "p4",
		"-profile:v", "high",
		"-rc", "constqp",
		"-qp", qp,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	f.currentCmd = cmd

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if f.cancelFlag {
			return fmt.Errorf("export cancelled")
		}
		return fmt.Errorf("nvenc export failed: %s", stderr.String())
	}

	return nil
}

// exportFullGameCPU exports using software encoding with filter_complex
func (f *FFmpeg) exportFullGameCPU(inputPaths []string, metaFile, outputPath, crf string) error {
	args := []string{}

	// Add each input file
	for _, path := range inputPaths {
		args = append(args, "-i", path)
	}

	// Add metadata file as last input
	args = append(args, "-i", metaFile)

	// Build filter_complex: scale each video to 1920x1080, then concat
	// This handles DNxHR MOV files with unknown streams and different resolutions
	filterStr := ""
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2,setsar=1[v%d];", i, i)
	}
	for i := range inputPaths {
		filterStr += fmt.Sprintf("[v%d][%d:a]", i, i)
	}
	filterStr += fmt.Sprintf("concat=n=%d:v=1:a=1[outv][outa]", len(inputPaths))

	args = append(args,
		"-filter_complex", filterStr,
		"-map", "[outv]",
		"-map", "[outa]",
		"-map_metadata", fmt.Sprintf("%d", len(inputPaths)),
		"-map_chapters", fmt.Sprintf("%d", len(inputPaths)),
		"-c:v", "libx264",
		"-preset", "medium",
		"-profile:v", "high",
		"-crf", crf,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-ar", "48000",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	f.currentCmd = cmd

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if f.cancelFlag {
			return fmt.Errorf("export cancelled")
		}
		return fmt.Errorf("cpu export failed: %s", stderr.String())
	}

	return nil
}
