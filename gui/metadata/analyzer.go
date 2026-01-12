package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopro-gui/ffmpeg"
)

// AnalysisResult contains all analyzed chapters with metadata
type AnalysisResult struct {
	Periods  []Period  `json:"periods"`
	Chapters []Chapter `json:"chapters"`
}

// Analyzer handles the analysis of GoPro footage
type Analyzer struct {
	ff *ffmpeg.FFmpeg
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer(ff *ffmpeg.FFmpeg) *Analyzer {
	return &Analyzer{ff: ff}
}

// AnalyzePeriods processes multiple periods and returns all chapters with clock times
func (a *Analyzer) AnalyzePeriods(periods []Period) (*AnalysisResult, error) {
	periodChapters := make(map[string][]Chapter)

	for _, period := range periods {
		var chapters []Chapter
		var err error

		// Parse the metadata file for chapters
		// If using MOV metadata and MetadataFile points to a video file, extract directly
		if period.UseMovMetadata && period.MetadataFile == period.VideoFile {
			// Extract chapters directly from the MOV file
			chapters, err = a.extractChaptersFromVideo(period.VideoFile)
		} else {
			chapters, err = ParseFFMetadata(period.MetadataFile)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata for %s: %w", period.Name, err)
		}

		if len(chapters) == 0 {
			continue // No chapters in this period
		}

		// Get the timecode - use GetTimecodeFromVideo for MOV files, GetTimecode for original GoPro
		var timecode string
		if period.UseMovMetadata {
			timecode, err = a.ff.GetTimecodeFromVideo(period.SourceGoPro)
		} else {
			timecode, err = a.ff.GetTimecode(period.SourceGoPro)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get timecode for %s: %w", period.Name, err)
		}

		// Map chapters to clock times
		mappedChapters, err := MapChaptersToClockTime(chapters, timecode)
		if err != nil {
			return nil, fmt.Errorf("failed to map chapters for %s: %w", period.Name, err)
		}

		periodChapters[period.Name] = mappedChapters
	}

	// Merge and sort all chapters
	allChapters := MergeAndSortChapters(periodChapters)

	return &AnalysisResult{
		Periods:  periods,
		Chapters: allChapters,
	}, nil
}

// SaveToJSON saves the analysis result to a JSON file
func (result *AnalysisResult) SaveToJSON(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// LoadFromJSON loads an analysis result from a JSON file
func LoadFromJSON(path string) (*AnalysisResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var result AnalysisResult
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return &result, nil
}

// GetPeriodVideoFile returns the video file path for a given period name
func (result *AnalysisResult) GetPeriodVideoFile(periodName string) string {
	for _, p := range result.Periods {
		if p.Name == periodName {
			return p.VideoFile
		}
	}
	return ""
}

// extractChaptersFromVideo extracts chapter markers directly from a video file using ffprobe
func (a *Analyzer) extractChaptersFromVideo(videoPath string) ([]Chapter, error) {
	// Create a temporary metadata file
	tempFile, err := os.CreateTemp("", "ffmeta-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempPath)

	// Extract metadata to temp file
	if err := a.ff.ExtractMetadata(videoPath, tempPath); err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Parse the temp file
	return ParseFFMetadata(tempPath)
}

// ChapterJSON is a JSON-friendly version of Chapter for serialization
type ChapterJSON struct {
	Number      int    `json:"number"`
	StartMs     int64  `json:"start_ms"`
	VideoTime   string `json:"video_time"`
	ClockTime   string `json:"clock_time"`
	GlobalOrder int    `json:"global_order"`
	Period      string `json:"period"`
}

// MarshalJSON implements custom JSON marshaling for Chapter
func (c Chapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(ChapterJSON{
		Number:      c.Number,
		StartMs:     c.StartMs,
		VideoTime:   FormatVideoTime(c.VideoTime),
		ClockTime:   c.ClockTime.Format("15:04:05.000"),
		GlobalOrder: c.GlobalOrder,
		Period:      c.Period,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for Chapter
func (c *Chapter) UnmarshalJSON(data []byte) error {
	var cj ChapterJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return err
	}

	c.Number = cj.Number
	c.StartMs = cj.StartMs
	c.GlobalOrder = cj.GlobalOrder
	c.Period = cj.Period

	// Parse video time (MM:SS format)
	var minutes, seconds int
	fmt.Sscanf(cj.VideoTime, "%d:%d", &minutes, &seconds)
	c.VideoTime = (time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second)

	// Parse clock time
	c.ClockTime, _ = time.Parse("15:04:05.000", cj.ClockTime)

	return nil
}
