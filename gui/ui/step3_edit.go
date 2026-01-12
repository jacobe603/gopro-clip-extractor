package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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

// createStep3Edit creates the clip editing UI
func (a *App) createStep3Edit() fyne.CanvasObject {
	var clipEntries []*clipEditEntry
	clipsContainer := container.NewVBox()

	statusLabel := widget.NewLabel("")

	// Helper to match clip filename to chapter (handles both .mp4 and .mov)
	matchClipToChapter := func(clipName string) *metadata.Chapter {
		// Remove extension for comparison
		clipBase := strings.TrimSuffix(clipName, filepath.Ext(clipName))

		for i := range a.analysisResult.Chapters {
			ch := &a.analysisResult.Chapters[i]
			expectedName := metadata.GenerateClipFilename(*ch)
			expectedBase := strings.TrimSuffix(expectedName, filepath.Ext(expectedName))
			if clipBase == expectedBase {
				return ch
			}
		}
		return nil
	}

	// Refresh clips list from extracted clips
	refreshClips := func() {
		clipsContainer.Objects = nil
		clipEntries = nil

		if len(a.extractedClips) == 0 || a.analysisResult == nil {
			clipsContainer.Add(widget.NewLabel("No clips available. Complete Step 2 first, or use 'Load from Folder'."))
			clipsContainer.Refresh()
			return
		}

		// Match extracted clips to chapters by filename
		for _, clipPath := range a.extractedClips {
			clipName := filepath.Base(clipPath)

			// Find matching chapter (handles both .mp4 and .mov)
			matchedChapter := matchClipToChapter(clipName)

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
			clipsContainer.Add(widget.NewLabel("No clips found. Extract clips in Step 2 first."))
		}

		clipsContainer.Refresh()
	}

	refreshBtn := widget.NewButton("Refresh", func() {
		refreshClips()
	})

	// Load clips from a folder (for when clips were extracted in a previous session)
	loadFromFolderBtn := widget.NewButton("Load from Folder", func() {
		if a.analysisResult == nil {
			a.showError("No Analysis", "Please complete Step 1 first to analyze periods")
			return
		}

		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			folderPath := uri.Path()
			if len(folderPath) > 2 && folderPath[0] == '/' && folderPath[2] == ':' {
				folderPath = folderPath[1:]
			}

			// Scan folder for clip files (.mp4 and .mov)
			entries, err := os.ReadDir(folderPath)
			if err != nil {
				a.showError("Error", "Failed to read folder: "+err.Error())
				return
			}

			a.extractedClips = nil
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext == ".mp4" || ext == ".mov" {
					clipPath := filepath.Join(folderPath, entry.Name())
					// Only add if it matches a chapter
					if matchClipToChapter(entry.Name()) != nil {
						a.extractedClips = append(a.extractedClips, clipPath)
					}
				}
			}

			statusLabel.SetText(fmt.Sprintf("Loaded %d clips from folder", len(a.extractedClips)))
			refreshClips()
		}, a.window)
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
				a.reExtractClipSync(ce)
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
		widget.NewLabel("Step 3: Edit Clips"),
		widget.NewSeparator(),
		helpText,
		container.NewHBox(refreshBtn, loadFromFolderBtn, reExtractAllBtn),
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		statusLabel,
	)

	return container.NewBorder(header, footer, nil, nil, scroll)
}

// reExtractClip re-extracts a single clip with updated timing (runs async for UI responsiveness)
func (a *App) reExtractClip(ce *clipEditEntry) {
	// Immediately show "Extracting..." status
	ce.statusLabel.SetText("Extracting...")
	ce.statusLabel.Refresh()

	// Run extraction in background so UI stays responsive
	go func() {
		a.doExtractClip(ce)
	}()
}

// reExtractClipSync re-extracts a single clip synchronously (for batch operations)
func (a *App) reExtractClipSync(ce *clipEditEntry) {
	// Show "Extracting..." status
	fyne.Do(func() {
		ce.statusLabel.SetText("Extracting...")
		ce.statusLabel.Refresh()
	})

	a.doExtractClip(ce)
}

// doExtractClip performs the actual extraction work
func (a *App) doExtractClip(ce *clipEditEntry) {
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

	// Extract the clip (overwrites existing)
	err = a.ff.ExtractClip(videoFile, ce.clipPath, startSec, duration)

	// Show completion with timestamp so user knows it's a fresh extraction
	fyne.Do(func() {
		if err != nil {
			ce.statusLabel.SetText("Error: " + err.Error())
		} else {
			timestamp := time.Now().Format("15:04:05")
			ce.statusLabel.SetText(fmt.Sprintf("Done! (%s)", timestamp))
		}
		ce.statusLabel.Refresh()
	})
}
