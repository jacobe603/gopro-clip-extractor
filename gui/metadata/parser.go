package metadata

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Chapter represents a single chapter/highlight marker in a video
type Chapter struct {
	Number      int           // Chapter number within its period
	StartMs     int64         // Start time in milliseconds from video start
	VideoTime   time.Duration // Time offset in the video
	ClockTime   time.Time     // Real-world clock time (from GoPro timecode)
	GlobalOrder int           // Order across all periods
	Period      string        // Period name
}

// Period represents a recording period with associated files
type Period struct {
	Name         string
	VideoFile    string
	MetadataFile string
	SourceGoPro  string
}

// ParseFFMetadata parses an FFmpeg metadata file and extracts chapter markers
func ParseFFMetadata(path string) ([]Chapter, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer file.Close()

	var chapters []Chapter
	chapterNum := 0

	// Regex to match START= lines
	startRe := regexp.MustCompile(`^START=(\d+)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if matches := startRe.FindStringSubmatch(line); matches != nil {
			startMs, _ := strconv.ParseInt(matches[1], 10, 64)
			chapterNum++

			chapters = append(chapters, Chapter{
				Number:    chapterNum,
				StartMs:   startMs,
				VideoTime: time.Duration(startMs) * time.Millisecond,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading metadata file: %w", err)
	}

	return chapters, nil
}

// ParseTimecodeToTime parses a GoPro timecode string and returns a time.Time
// Assumes the timecode represents time of day in the local timezone
func ParseTimecodeToTime(timecode string) (time.Time, error) {
	// Match HH:MM:SS:FF or HH:MM:SS;FF
	re := regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[:;](\d{2})`)
	matches := re.FindStringSubmatch(timecode)
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid timecode format: %s", timecode)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	frames, _ := strconv.Atoi(matches[4])

	// Convert frames to milliseconds (assuming ~60fps)
	const fps = 60.0
	milliseconds := int(float64(frames) / fps * 1000)

	// Create a time using today's date (we only care about time of day)
	now := time.Now()
	return time.Date(
		now.Year(), now.Month(), now.Day(),
		hours, minutes, seconds, milliseconds*1e6,
		time.Local,
	), nil
}

// TimecodeToSeconds converts a timecode string to total seconds
func TimecodeToSeconds(timecode string) (float64, error) {
	re := regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[:;](\d{2})`)
	matches := re.FindStringSubmatch(timecode)
	if matches == nil {
		return 0, fmt.Errorf("invalid timecode format: %s", timecode)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	frames, _ := strconv.Atoi(matches[4])

	const fps = 60.0
	return float64(hours*3600+minutes*60+seconds) + float64(frames)/fps, nil
}

// MapChaptersToClockTime maps chapter video times to real clock times
// using the GoPro timecode as the reference point
func MapChaptersToClockTime(chapters []Chapter, goProTimecode string) ([]Chapter, error) {
	startTime, err := ParseTimecodeToTime(goProTimecode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GoPro timecode: %w", err)
	}

	result := make([]Chapter, len(chapters))
	for i, ch := range chapters {
		result[i] = ch
		result[i].ClockTime = startTime.Add(ch.VideoTime)
	}

	return result, nil
}

// MergeAndSortChapters combines chapters from multiple periods and sorts them chronologically
func MergeAndSortChapters(periodChapters map[string][]Chapter) []Chapter {
	var allChapters []Chapter

	for periodName, chapters := range periodChapters {
		for _, ch := range chapters {
			ch.Period = periodName
			allChapters = append(allChapters, ch)
		}
	}

	// Sort by clock time
	sort.Slice(allChapters, func(i, j int) bool {
		return allChapters[i].ClockTime.Before(allChapters[j].ClockTime)
	})

	// Assign global order
	for i := range allChapters {
		allChapters[i].GlobalOrder = i + 1
	}

	return allChapters
}

// FormatClockTime formats a time as HH-MM-SS-mmm for filenames
func FormatClockTime(t time.Time) string {
	return fmt.Sprintf("%02d-%02d-%02d-%03d",
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1e6)
}

// FormatVideoTime formats a duration as MM:SS for display
func FormatVideoTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// GenerateClipFilename generates a filename for an extracted clip
func GenerateClipFilename(ch Chapter) string {
	return fmt.Sprintf("%03d_%s_%s_Ch%02d.mp4",
		ch.GlobalOrder,
		FormatClockTime(ch.ClockTime),
		sanitizeFilename(ch.Period),
		ch.Number,
	)
}

// sanitizeFilename removes characters that are invalid in filenames
func sanitizeFilename(name string) string {
	// Replace spaces with underscores and remove invalid characters
	name = strings.ReplaceAll(name, " ", "_")
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	return re.ReplaceAllString(name, "")
}
