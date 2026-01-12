package ui

import (
	"fmt"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

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

		progressBar.Show()
		progressBar.SetValue(0)
		a.extractedClips = []string{}

		go func() {
			totalClips := len(toExtract)
			completedClips := 0

			for _, ch := range toExtract {
				// Capture values for this iteration
				currentClip := completedClips + 1
				periodName := ch.Period
				chapterNum := ch.Number

				// Update status BEFORE starting extraction
				fyne.Do(func() {
					progressBar.SetValue(float64(completedClips) / float64(totalClips))
					statusLabel.SetText(fmt.Sprintf("Extracting %d/%d: %s Ch%d...",
						currentClip, totalClips, periodName, chapterNum))
				})

				// Get video file for this chapter's period
				videoFile := a.analysisResult.GetPeriodVideoFile(ch.Period)
				if videoFile == "" {
					fyne.Do(func() {
						statusLabel.SetText(fmt.Sprintf("Error: No video file for period %s", periodName))
					})
					continue
				}

				// Calculate clip timing
				startSec := ch.VideoTime.Seconds() - secBefore
				if startSec < 0 {
					startSec = 0
				}
				duration := secBefore + secAfter

				// Generate output filename with appropriate extension
				clipName := metadata.GenerateClipFilename(ch)
				if streamCopyCheck.Checked {
					// Change extension to .mov for stream copy
					clipName = clipName[:len(clipName)-4] + ".mov"
				}
				outputFile := filepath.Join(outputFolder, clipName)

				// Extract the clip using appropriate method
				var err error
				if streamCopyCheck.Checked {
					err = a.ff.ExtractClipStreamCopy(videoFile, outputFile, startSec, duration)
				} else {
					err = a.ff.ExtractClip(videoFile, outputFile, startSec, duration)
				}
				if err != nil {
					fyne.Do(func() {
						statusLabel.SetText(fmt.Sprintf("Error extracting Ch%d: %s", chapterNum, err.Error()))
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
				statusLabel.SetText(fmt.Sprintf("Done! Extracted %d clips to %s", finalCount, outputFolder))
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
