package metadata

import (
	"fmt"
	"sort"
	"time"
)

// ClipGroup represents a group of one or more chapters that will be extracted as a single clip.
// When multiple chapters are close together (their clips would overlap), they are merged
// into a single ClipGroup to avoid repeated video content.
//
// FUTURE EXTENSION (Option B): This struct can be extended to support user choice:
// - Add a `Merged bool` field to track if user chose to merge or keep separate
// - Add a `OriginalChapters` field to preserve the original chapters for UI display
// - Add a `RecommendedAfterTime` field to suggest timing adjustments
type ClipGroup struct {
	// Chapters contains all chapters in this group.
	// For non-overlapping chapters, this will have exactly one element.
	// For overlapping chapters, this contains all merged chapters sorted by time.
	Chapters []Chapter

	// StartTime is the extraction start time in the video (seconds).
	// Calculated as: first chapter's VideoTime - beforePadding
	StartTime float64

	// EndTime is the extraction end time in the video (seconds).
	// Calculated as: last chapter's VideoTime + afterPadding
	EndTime float64

	// Duration is the total clip duration in seconds (EndTime - StartTime).
	Duration float64

	// Period is the period name (all chapters in a group must be from the same period).
	Period string

	// PrimaryChapter is the first chapter in the group, used for naming the output file.
	PrimaryChapter Chapter

	// IsOverlap indicates whether this group contains multiple merged chapters.
	// FUTURE EXTENSION (Option B): Use this to show overlap warnings in UI
	IsOverlap bool

	// OverlapInfo contains human-readable info about the overlap for UI display.
	// FUTURE EXTENSION (Option B): Display this in UI to let user choose merge vs separate
	OverlapInfo string
}

// DetectOverlappingChapters analyzes chapters and groups overlapping ones together.
// Two chapters overlap when the clip from the first chapter (with padding) would
// include video that also appears in the clip from the second chapter.
//
// Parameters:
//   - chapters: List of chapters to analyze (should be from the same period or sorted by time)
//   - beforePadding: Seconds to include before each chapter marker
//   - afterPadding: Seconds to include after each chapter marker
//
// Returns:
//   - []ClipGroup: Groups of chapters, where overlapping chapters are merged
//
// FUTURE EXTENSION (Option B): Add a `mergeOverlaps bool` parameter to control behavior:
//   - When true (current behavior): automatically merge overlapping chapters
//   - When false: keep chapters separate but populate OverlapInfo for UI warnings
func DetectOverlappingChapters(chapters []Chapter, beforePadding, afterPadding float64) []ClipGroup {
	if len(chapters) == 0 {
		return nil
	}

	// Group chapters by period first, since we can't merge across periods
	// (different video files)
	periodChapters := make(map[string][]Chapter)
	for _, ch := range chapters {
		periodChapters[ch.Period] = append(periodChapters[ch.Period], ch)
	}

	var allGroups []ClipGroup

	// Process each period separately
	for period, pChapters := range periodChapters {
		// Sort chapters by video time within this period
		sorted := make([]Chapter, len(pChapters))
		copy(sorted, pChapters)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].VideoTime < sorted[j].VideoTime
		})

		// Build groups by detecting overlaps
		groups := buildOverlapGroups(sorted, beforePadding, afterPadding, period)
		allGroups = append(allGroups, groups...)
	}

	// Sort all groups by the primary chapter's global order for consistent output
	sort.Slice(allGroups, func(i, j int) bool {
		return allGroups[i].PrimaryChapter.GlobalOrder < allGroups[j].PrimaryChapter.GlobalOrder
	})

	return allGroups
}

// buildOverlapGroups creates ClipGroups from a sorted list of chapters from the same period.
// This is the core overlap detection algorithm.
//
// Algorithm:
//  1. Start with the first chapter as the current group
//  2. For each subsequent chapter, check if its clip would overlap with the current group
//  3. If overlap: extend the current group to include this chapter
//  4. If no overlap: finalize the current group and start a new one
//
// Overlap condition:
//
//	next_chapter_start - beforePadding < current_group_end
//	Which simplifies to: next_chapter_time < current_group_end + beforePadding
func buildOverlapGroups(sortedChapters []Chapter, beforePadding, afterPadding float64, period string) []ClipGroup {
	if len(sortedChapters) == 0 {
		return nil
	}

	var groups []ClipGroup

	// Initialize first group with first chapter
	currentGroup := ClipGroup{
		Chapters:       []Chapter{sortedChapters[0]},
		StartTime:      maxFloat(0, sortedChapters[0].VideoTime.Seconds()-beforePadding),
		EndTime:        sortedChapters[0].VideoTime.Seconds() + afterPadding,
		Period:         period,
		PrimaryChapter: sortedChapters[0],
		IsOverlap:      false,
	}

	// Process remaining chapters
	for i := 1; i < len(sortedChapters); i++ {
		ch := sortedChapters[i]
		chStartTime := maxFloat(0, ch.VideoTime.Seconds()-beforePadding)

		// Check for overlap: does this chapter's clip start before the current group ends?
		if chStartTime < currentGroup.EndTime {
			// Overlap detected - merge into current group
			currentGroup.Chapters = append(currentGroup.Chapters, ch)
			currentGroup.EndTime = ch.VideoTime.Seconds() + afterPadding
			currentGroup.IsOverlap = true
		} else {
			// No overlap - finalize current group and start new one
			finalizeGroup(&currentGroup)
			groups = append(groups, currentGroup)

			currentGroup = ClipGroup{
				Chapters:       []Chapter{ch},
				StartTime:      chStartTime,
				EndTime:        ch.VideoTime.Seconds() + afterPadding,
				Period:         period,
				PrimaryChapter: ch,
				IsOverlap:      false,
			}
		}
	}

	// Don't forget the last group
	finalizeGroup(&currentGroup)
	groups = append(groups, currentGroup)

	return groups
}

// finalizeGroup calculates final values for a ClipGroup after all chapters are added.
func finalizeGroup(group *ClipGroup) {
	group.Duration = group.EndTime - group.StartTime

	// Build overlap info string for UI display
	// FUTURE EXTENSION (Option B): Use this info to show warnings and let user choose
	if group.IsOverlap {
		chapterNums := make([]int, len(group.Chapters))
		for i, ch := range group.Chapters {
			chapterNums[i] = ch.GlobalOrder
		}

		// Calculate how much time between first and last chapter
		firstTime := group.Chapters[0].VideoTime
		lastTime := group.Chapters[len(group.Chapters)-1].VideoTime
		gap := lastTime - firstTime

		group.OverlapInfo = fmt.Sprintf(
			"Merged %d highlights (%.1fs apart) into %.1fs clip",
			len(group.Chapters),
			gap.Seconds(),
			group.Duration,
		)
	}
}

// GenerateGroupFilename creates an output filename for a ClipGroup.
// For single-chapter groups, uses the standard naming.
// For merged groups, indicates the range of chapters included.
// Format matches standard clip naming for proper sort order:
//
//	{GlobalOrder}_{ClockTime}_{Period}_Ch{First}-{Last}.mp4
//	Example: 041_12-15-45-871_3Period_Ch05-06.mp4
func GenerateGroupFilename(group ClipGroup) string {
	if !group.IsOverlap {
		// Single chapter - use standard naming
		return GenerateClipFilename(group.PrimaryChapter)
	}

	// Merged group - include range in filename
	// Use same format as single clips for proper sorting
	first := group.PrimaryChapter
	last := group.Chapters[len(group.Chapters)-1]

	return fmt.Sprintf("%03d_%s_%s_Ch%02d-%02d.mp4",
		first.GlobalOrder,
		FormatClockTime(first.ClockTime),
		sanitizeFilename(first.Period),
		first.Number,
		last.Number,
	)
}

// GetOverlapSummary returns a summary string describing all overlaps detected.
// Useful for displaying in the UI before extraction.
//
// FUTURE EXTENSION (Option B): Expand this to return structured data for UI display,
// allowing users to select which overlaps to merge vs keep separate.
func GetOverlapSummary(groups []ClipGroup) string {
	var overlapCount int
	var totalMerged int

	for _, g := range groups {
		if g.IsOverlap {
			overlapCount++
			totalMerged += len(g.Chapters)
		}
	}

	if overlapCount == 0 {
		return ""
	}

	return fmt.Sprintf("%d overlapping highlight groups detected (%d highlights merged into %d clips)",
		overlapCount, totalMerged, overlapCount)
}

// maxFloat returns the maximum of two float64 values.
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ClipChapterInfo holds chapter timing information for embedding in extracted clips.
// Each chapter marks the beginning of a highlight within the merged clip.
type ClipChapterInfo struct {
	// OffsetMs is the chapter position in milliseconds from the start of the clip
	OffsetMs int64
	// Title is the chapter title (e.g., "Highlight 1", "Ch03")
	Title string
}

// GetClipChapters returns chapter markers for embedding in the extracted clip.
// Each chapter marks the beginning of a highlight within the merged clip.
// For single-chapter clips, returns one chapter at 0ms (clip start).
// For merged clips, returns chapters at the offset of each original highlight.
func (g *ClipGroup) GetClipChapters() []ClipChapterInfo {
	var chapters []ClipChapterInfo

	for i, ch := range g.Chapters {
		// Calculate offset from clip start to this highlight
		// Clip starts at g.StartTime, highlight is at ch.VideoTime
		offsetSec := ch.VideoTime.Seconds() - g.StartTime
		if offsetSec < 0 {
			offsetSec = 0
		}

		title := fmt.Sprintf("Highlight %d (Ch%02d)", i+1, ch.Number)
		if !g.IsOverlap {
			title = fmt.Sprintf("Ch%02d", ch.Number)
		}

		chapters = append(chapters, ClipChapterInfo{
			OffsetMs: int64(offsetSec * 1000),
			Title:    title,
		})
	}

	return chapters
}

// CalculateRecommendedAfterTime calculates what the "after" time should be for a chapter
// to extend its clip to cover a subsequent overlapping chapter.
// This is useful for Option B where users might want to manually adjust timing.
//
// Parameters:
//   - chapter1Time: VideoTime of the first chapter
//   - chapter2Time: VideoTime of the second (overlapping) chapter
//   - originalAfter: The original after-padding value
//
// Returns:
//   - Recommended after-padding that would make chapter1's clip cover chapter2
//
// FUTURE EXTENSION (Option B): Use this to show recommended timing adjustments in UI
func CalculateRecommendedAfterTime(chapter1Time, chapter2Time time.Duration, originalAfter float64) float64 {
	gap := chapter2Time.Seconds() - chapter1Time.Seconds()
	return gap + originalAfter
}
