package ui

import (
	"fmt"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"gopro-gui/ffmpeg"
	"gopro-gui/metadata"
)

// createStep2Extract creates the clip extraction UI
func (a *App) createStep2Extract() fyne.CanvasObject {
	// Chapter selection
	selectedChapters := make(map[int]bool)
	var checkboxes []*widget.Check
	chaptersContainer := container.NewVBox()

	// Output folder
	outputFolderLabel := widget.NewLabel("(none selected)")
	var outputFolder string

	// Timing settings
	beforeEntry := widget.NewEntry()
	beforeEntry.SetText(fmt.Sprintf("%.0f", a.cfg.SecondsBefore))
	afterEntry := widget.NewEntry()
	afterEntry.SetText(fmt.Sprintf("%.0f", a.cfg.SecondsAfter))

	// Encoding mode
	streamCopyCheck := widget.NewCheck("Stream copy (MOV for Shotcut/editing) - Fast, no re-encoding", nil)
	streamCopyCheck.SetChecked(false) // Default to re-encode for YouTube

	// Status
	statusLabel := widget.NewLabel("")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Refresh chapters list
	refreshChapters := func() {
		chaptersContainer.Objects = nil
		checkboxes = nil
		selectedChapters = make(map[int]bool)

		if a.analysisResult == nil || len(a.analysisResult.Chapters) == 0 {
			chaptersContainer.Add(widget.NewLabel("No chapters available. Complete Step 1 first."))
			chaptersContainer.Refresh()
			return
		}

		for _, ch := range a.analysisResult.Chapters {
			ch := ch // capture for closure
			check := widget.NewCheck(
				fmt.Sprintf("%03d. [%s] %s Ch%02d @ %s",
					ch.GlobalOrder,
					ch.Period,
					ch.ClockTime.Format("15:04:05"),
					ch.Number,
					metadata.FormatVideoTime(ch.VideoTime),
				),
				func(checked bool) {
					selectedChapters[ch.GlobalOrder] = checked
				},
			)
			check.SetChecked(true)
			selectedChapters[ch.GlobalOrder] = true
			checkboxes = append(checkboxes, check)
			chaptersContainer.Add(check)
		}
		chaptersContainer.Refresh()
	}

	refreshBtn := widget.NewButton("Refresh Chapters", func() {
		refreshChapters()
	})

	selectAllBtn := widget.NewButton("Select All", func() {
		for i, cb := range checkboxes {
			cb.SetChecked(true)
			if a.analysisResult != nil && i < len(a.analysisResult.Chapters) {
				selectedChapters[a.analysisResult.Chapters[i].GlobalOrder] = true
			}
		}
	})

	deselectAllBtn := widget.NewButton("Deselect All", func() {
		for i, cb := range checkboxes {
			cb.SetChecked(false)
			if a.analysisResult != nil && i < len(a.analysisResult.Chapters) {
				selectedChapters[a.analysisResult.Chapters[i].GlobalOrder] = false
			}
		}
	})

	selectOutputBtn := widget.NewButton("Select Output Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			outputFolder = path
			outputFolderLabel.SetText(path)
			a.cfg.LastOutputDir = path
		}, a.window)
	})

	extractBtn := widget.NewButton("Extract Selected Clips", func() {
		if a.analysisResult == nil || len(a.analysisResult.Chapters) == 0 {
			a.showError("No Chapters", "Please complete Step 1 first to analyze chapters")
			return
		}

		if outputFolder == "" {
			a.showError("No Output Folder", "Please select an output folder")
			return
		}

		// Parse timing
		secBefore, err := strconv.ParseFloat(beforeEntry.Text, 64)
		if err != nil {
			secBefore = 8.0
		}
		secAfter, err := strconv.ParseFloat(afterEntry.Text, 64)
		if err != nil {
			secAfter = 2.0
		}
		a.cfg.SecondsBefore = secBefore
		a.cfg.SecondsAfter = secAfter

		// Get selected chapters
		var toExtract []metadata.Chapter
		for _, ch := range a.analysisResult.Chapters {
			if selectedChapters[ch.GlobalOrder] {
				toExtract = append(toExtract, ch)
			}
		}

		if len(toExtract) == 0 {
			a.showError("No Selection", "Please select at least one chapter to extract")
			return
		}

		// Detect and merge overlapping chapters to avoid repeated video content
		// FUTURE EXTENSION (Option B): Add UI to let user choose merge vs separate
		clipGroups := metadata.DetectOverlappingChapters(toExtract, secBefore, secAfter)

		// Show overlap summary if any overlaps were detected
		overlapSummary := metadata.GetOverlapSummary(clipGroups)
		if overlapSummary != "" {
			statusLabel.SetText(overlapSummary)
		}

		progressBar.Show()
		progressBar.SetValue(0)
		a.extractedClips = []string{}

		go func() {
			totalClips := len(clipGroups)
			completedClips := 0

			for _, group := range clipGroups {
				// Capture values for this iteration
				currentClip := completedClips + 1
				periodName := group.Period

				// Build status message based on whether this is a merged group
				var statusMsg string
				if group.IsOverlap {
					statusMsg = fmt.Sprintf("Extracting %d/%d: %s Ch%d-%d (merged, %.1fs)...",
						currentClip, totalClips, periodName,
						group.PrimaryChapter.Number,
						group.Chapters[len(group.Chapters)-1].Number,
						group.Duration)
				} else {
					statusMsg = fmt.Sprintf("Extracting %d/%d: %s Ch%d...",
						currentClip, totalClips, periodName, group.PrimaryChapter.Number)
				}

				// Update status BEFORE starting extraction
				fyne.Do(func() {
					progressBar.SetValue(float64(completedClips) / float64(totalClips))
					statusLabel.SetText(statusMsg)
				})

				// Get video file for this group's period
				videoFile := a.analysisResult.GetPeriodVideoFile(group.Period)
				if videoFile == "" {
					fyne.Do(func() {
						statusLabel.SetText(fmt.Sprintf("Error: No video file for period %s", periodName))
					})
					continue
				}

				// Use pre-calculated timing from the ClipGroup
				startSec := group.StartTime
				duration := group.Duration

				// Get chapter markers for this clip
				clipChapterInfo := group.GetClipChapters()
				var chapters []ffmpeg.ClipChapter
				for _, ch := range clipChapterInfo {
					chapters = append(chapters, ffmpeg.ClipChapter{
						OffsetMs: ch.OffsetMs,
						Title:    ch.Title,
					})
				}

				// Generate output filename with appropriate extension
				clipName := metadata.GenerateGroupFilename(group)
				if streamCopyCheck.Checked {
					// Change extension to .mov for stream copy
					clipName = clipName[:len(clipName)-4] + ".mov"
				}
				outputFile := filepath.Join(outputFolder, clipName)

				// Extract the clip with chapter markers embedded
				var err error
				if streamCopyCheck.Checked {
					err = a.ff.ExtractClipStreamCopyWithChapters(videoFile, outputFile, startSec, duration, chapters)
				} else {
					err = a.ff.ExtractClipWithChapters(videoFile, outputFile, startSec, duration, chapters)
				}
				if err != nil {
					fyne.Do(func() {
						statusLabel.SetText(fmt.Sprintf("Error extracting: %s", err.Error()))
					})
				} else {
					a.extractedClips = append(a.extractedClips, outputFile)
					completedClips++
				}
			}

			finalCount := len(a.extractedClips)
			fyne.Do(func() {
				progressBar.SetValue(1.0)
				progressBar.Hide()
				var doneMsg string
				if overlapSummary != "" {
					doneMsg = fmt.Sprintf("Done! Extracted %d clips (%s)", finalCount, overlapSummary)
				} else {
					doneMsg = fmt.Sprintf("Done! Extracted %d clips to %s", finalCount, outputFolder)
				}
				statusLabel.SetText(doneMsg)
				// Mark step complete if we extracted at least one clip
				if finalCount > 0 {
					a.markStepComplete(1)
				}
			})
		}()
	})

	// Initial refresh
	refreshChapters()

	// Layout
	timingRow := container.NewHBox(
		widget.NewLabel("Seconds before:"),
		beforeEntry,
		widget.NewLabel("Seconds after:"),
		afterEntry,
	)

	encodingRow := container.NewVBox(
		streamCopyCheck,
		widget.NewLabel("  Unchecked = Re-encode to MP4 (H.264) for YouTube"),
	)

	selectionBtns := container.NewHBox(refreshBtn, selectAllBtn, deselectAllBtn)

	outputRow := container.NewHBox(
		widget.NewLabel("Output folder:"),
		outputFolderLabel,
		selectOutputBtn,
	)

	scroll := container.NewScroll(chaptersContainer)
	scroll.SetMinSize(fyne.NewSize(0, 300))

	return container.NewVBox(
		widget.NewLabel("Step 2: Select and Extract Clips"),
		widget.NewSeparator(),
		timingRow,
		encodingRow,
		widget.NewSeparator(),
		widget.NewLabel("Select chapters to extract:"),
		selectionBtns,
		scroll,
		widget.NewSeparator(),
		outputRow,
		extractBtn,
		statusLabel,
		progressBar,
	)
}
