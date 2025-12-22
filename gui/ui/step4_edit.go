package ui

import (
	"fmt"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"gopro-gui/metadata"
)

// clipEditEntry holds the UI elements for editing a single clip
type clipEditEntry struct {
	chapter     metadata.Chapter
	clipPath    string
	beforeEntry *widget.Entry
	afterEntry  *widget.Entry
	statusLabel *widget.Label
}

// createStep4Edit creates the clip editing UI
func (a *App) createStep4Edit() fyne.CanvasObject {
	var clipEntries []*clipEditEntry
	clipsContainer := container.NewVBox()

	statusLabel := widget.NewLabel("")

	// Refresh clips list from extracted clips
	refreshClips := func() {
		clipsContainer.Objects = nil
		clipEntries = nil

		if len(a.extractedClips) == 0 || a.analysisResult == nil {
			clipsContainer.Add(widget.NewLabel("No clips available. Complete Step 3 first to extract clips."))
			clipsContainer.Refresh()
			return
		}

		// Match extracted clips to chapters by filename
		for _, clipPath := range a.extractedClips {
			clipName := filepath.Base(clipPath)

			// Find matching chapter
			var matchedChapter *metadata.Chapter
			for i := range a.analysisResult.Chapters {
				ch := &a.analysisResult.Chapters[i]
				expectedName := metadata.GenerateClipFilename(*ch)
				if clipName == expectedName {
					matchedChapter = ch
					break
				}
			}

			if matchedChapter == nil {
				continue
			}

			ce := &clipEditEntry{
				chapter:     *matchedChapter,
				clipPath:    clipPath,
				beforeEntry: widget.NewEntry(),
				afterEntry:  widget.NewEntry(),
				statusLabel: widget.NewLabel(""),
			}

			// Set default values from config
			ce.beforeEntry.SetText(fmt.Sprintf("%.1f", a.cfg.SecondsBefore))
			ce.afterEntry.SetText(fmt.Sprintf("%.1f", a.cfg.SecondsAfter))

			clipEntries = append(clipEntries, ce)

			// Create the card for this clip
			ch := ce.chapter
			headerText := fmt.Sprintf("%03d. [%s] %s Ch%02d @ %s",
				ch.GlobalOrder,
				ch.Period,
				ch.ClockTime.Format("15:04:05"),
				ch.Number,
				metadata.FormatVideoTime(ch.VideoTime),
			)

			timingRow := container.NewHBox(
				widget.NewLabel("Before:"),
				ce.beforeEntry,
				widget.NewLabel("s"),
				widget.NewLabel("  After:"),
				ce.afterEntry,
				widget.NewLabel("s"),
			)

			// Make entries smaller
			ce.beforeEntry.Resize(fyne.NewSize(60, ce.beforeEntry.MinSize().Height))
			ce.afterEntry.Resize(fyne.NewSize(60, ce.afterEntry.MinSize().Height))

			reExtractBtn := widget.NewButton("Re-Extract", func() {
				// Capture the entry for this closure
				entry := ce
				a.reExtractClip(entry)
			})

			card := widget.NewCard(
				headerText,
				filepath.Base(ce.clipPath),
				container.NewVBox(
					timingRow,
					container.NewHBox(reExtractBtn, ce.statusLabel),
				),
			)

			clipsContainer.Add(card)
		}

		if len(clipEntries) == 0 {
			clipsContainer.Add(widget.NewLabel("No clips found. Extract clips in Step 3 first."))
		}

		clipsContainer.Refresh()
	}

	refreshBtn := widget.NewButton("Refresh Clip List", func() {
		refreshClips()
	})

	reExtractAllBtn := widget.NewButton("Re-Extract All With New Timings", func() {
		if len(clipEntries) == 0 {
			a.showError("No Clips", "No clips to re-extract")
			return
		}

		statusLabel.SetText("Re-extracting all clips...")

		go func() {
			for i, ce := range clipEntries {
				progress := fmt.Sprintf("Re-extracting %d/%d...", i+1, len(clipEntries))
				fyne.Do(func() {
					statusLabel.SetText(progress)
				})
				a.reExtractClip(ce)
			}

			fyne.Do(func() {
				statusLabel.SetText(fmt.Sprintf("Done! Re-extracted %d clips.", len(clipEntries)))
			})
		}()
	})

	// Initial refresh
	refreshClips()

	// Layout
	scroll := container.NewScroll(clipsContainer)
	scroll.SetMinSize(fyne.NewSize(0, 400))

	helpText := widget.NewLabel("Adjust the before/after timing for individual clips and re-extract them.\n" +
		"This will overwrite the existing clip files.")
	helpText.Wrapping = fyne.TextWrapWord

	header := container.NewVBox(
		widget.NewLabel("Step 4: Edit Clips"),
		widget.NewSeparator(),
		helpText,
		container.NewHBox(refreshBtn, reExtractAllBtn),
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		statusLabel,
	)

	return container.NewBorder(header, footer, nil, nil, scroll)
}

// reExtractClip re-extracts a single clip with updated timing
func (a *App) reExtractClip(ce *clipEditEntry) {
	// Parse timing values
	secBefore, err := strconv.ParseFloat(ce.beforeEntry.Text, 64)
	if err != nil {
		secBefore = a.cfg.SecondsBefore
	}
	secAfter, err := strconv.ParseFloat(ce.afterEntry.Text, 64)
	if err != nil {
		secAfter = a.cfg.SecondsAfter
	}

	// Get video file for this chapter's period
	videoFile := a.analysisResult.GetPeriodVideoFile(ce.chapter.Period)
	if videoFile == "" {
		fyne.Do(func() {
			ce.statusLabel.SetText("Error: No video file")
		})
		return
	}

	// Calculate clip timing
	startSec := ce.chapter.VideoTime.Seconds() - secBefore
	if startSec < 0 {
		startSec = 0
	}
	duration := secBefore + secAfter

	fyne.Do(func() {
		ce.statusLabel.SetText("Extracting...")
	})

	// Extract the clip (overwrites existing)
	err = a.ff.ExtractClip(videoFile, ce.clipPath, startSec, duration)

	fyne.Do(func() {
		if err != nil {
			ce.statusLabel.SetText("Error: " + err.Error())
		} else {
			ce.statusLabel.SetText("Done!")
		}
	})
}
