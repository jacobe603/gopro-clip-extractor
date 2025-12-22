package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"gopro-gui/metadata"
)

// detectedPeriod holds auto-detected file paths for a period
type detectedPeriod struct {
	videoPath  string // .MOV file (converted)
	metaPath   string // _metadata.txt file
	sourcePath string // .MP4 file (original GoPro)
}

// scanFolderForPeriods scans a folder and detects period files
func scanFolderForPeriods(folderPath string) []detectedPeriod {
	var results []detectedPeriod

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return results
	}

	// Find all .MOV files (converted videos)
	var movFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".mov" {
			movFiles = append(movFiles, filepath.Join(folderPath, name))
		}
	}

	// Sort alphabetically (GX01, GX02, GX03 order)
	sort.Strings(movFiles)

	// For each MOV, look for matching MP4 and metadata
	for _, movPath := range movFiles {
		baseName := strings.TrimSuffix(filepath.Base(movPath), filepath.Ext(movPath))

		dp := detectedPeriod{
			videoPath: movPath,
		}

		// Look for matching .MP4 (original GoPro)
		mp4Path := filepath.Join(folderPath, baseName+".MP4")
		if _, err := os.Stat(mp4Path); err == nil {
			dp.sourcePath = mp4Path
		} else {
			// Try lowercase
			mp4Path = filepath.Join(folderPath, baseName+".mp4")
			if _, err := os.Stat(mp4Path); err == nil {
				dp.sourcePath = mp4Path
			}
		}

		// Look for matching _metadata.txt
		metaPath := filepath.Join(folderPath, baseName+"_metadata.txt")
		if _, err := os.Stat(metaPath); err == nil {
			dp.metaPath = metaPath
		}

		results = append(results, dp)
	}

	return results
}

// periodEntry holds the UI elements for a single period
type periodEntry struct {
	nameEntry       *widget.Entry
	videoPathLabel  *widget.Label
	videoPath       string
	metaPathLabel   *widget.Label
	metaPath        string
	sourcePathLabel *widget.Label
	sourcePath      string
}

// createStep2 creates the period configuration UI
func (a *App) createStep2() fyne.CanvasObject {
	var periodEntries []*periodEntry
	periodsContainer := container.NewVBox()

	statusLabel := widget.NewLabel("")
	chaptersLabel := widget.NewLabel("")
	chaptersLabel.Wrapping = fyne.TextWrapWord

	createPeriodUI := func() *periodEntry {
		pe := &periodEntry{
			nameEntry:       widget.NewEntry(),
			videoPathLabel:  widget.NewLabel("(none selected)"),
			metaPathLabel:   widget.NewLabel("(none selected)"),
			sourcePathLabel: widget.NewLabel("(none selected)"),
		}
		pe.nameEntry.SetPlaceHolder("e.g., 1st Period")
		pe.nameEntry.SetMinRowsVisible(1)
		return pe
	}

	addPeriodCard := func() {
		pe := createPeriodUI()
		periodEntries = append(periodEntries, pe)
		periodNum := len(periodEntries)

		// Pre-populate the period name for sorting (1Period, 2Period, etc.)
		pe.nameEntry.SetText(fmt.Sprintf("%dPeriod", periodNum))

		// Use Border layout so the name entry expands to fill available space
		nameRow := container.NewBorder(nil, nil, widget.NewLabel("Name:        "), nil, pe.nameEntry)

		card := widget.NewCard(
			fmt.Sprintf("Period %d", periodNum),
			"",
			container.NewVBox(
				nameRow,
				container.NewHBox(
					widget.NewLabel("Converted Video:"),
					pe.videoPathLabel,
					widget.NewButton("Select (.MOV/.MP4)", func() {
						fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
							if err != nil || reader == nil {
								return
							}
							reader.Close()
							path := reader.URI().Path()
							if len(path) > 2 && path[0] == '/' && path[2] == ':' {
								path = path[1:]
							}
							pe.videoPath = path
							pe.videoPathLabel.SetText(filepath.Base(path))
						}, a.window)
						fd.SetFilter(storage.NewExtensionFileFilter([]string{".mov", ".MOV", ".mp4", ".MP4"}))
						fd.Show()
					}),
				),
				container.NewHBox(
					widget.NewLabel("Metadata File:"),
					pe.metaPathLabel,
					widget.NewButton("Select", func() {
						fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
							if err != nil || reader == nil {
								return
							}
							reader.Close()
							path := reader.URI().Path()
							if len(path) > 2 && path[0] == '/' && path[2] == ':' {
								path = path[1:]
							}
							pe.metaPath = path
							pe.metaPathLabel.SetText(filepath.Base(path))
						}, a.window)
						fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt", ".TXT"}))
						fd.Show()
					}),
				),
				container.NewHBox(
					widget.NewLabel("Original GoPro:"),
					pe.sourcePathLabel,
					widget.NewButton("Select (for timecode)", func() {
						fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
							if err != nil || reader == nil {
								return
							}
							reader.Close()
							path := reader.URI().Path()
							if len(path) > 2 && path[0] == '/' && path[2] == ':' {
								path = path[1:]
							}
							pe.sourcePath = path
							pe.sourcePathLabel.SetText(filepath.Base(path))
						}, a.window)
						fd.SetFilter(storage.NewExtensionFileFilter([]string{".mp4", ".MP4"}))
						fd.Show()
					}),
				),
			),
		)

		periodsContainer.Add(card)
	}

	// Add 3 periods by default (hockey periods)
	addPeriodCard()
	addPeriodCard()
	addPeriodCard()

	// Folder selection for auto-detection
	folderLabel := widget.NewLabel("(none selected)")
	var selectedFolder string

	selectFolderBtn := widget.NewButton("Select Folder (Auto-Detect)", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			selectedFolder = path
			folderLabel.SetText(path)

			// Scan folder and auto-fill periods
			detected := scanFolderForPeriods(path)

			// Fill in detected files for each period
			for i, pe := range periodEntries {
				if i < len(detected) {
					dp := detected[i]

					// Set video path
					if dp.videoPath != "" {
						pe.videoPath = dp.videoPath
						pe.videoPathLabel.SetText(filepath.Base(dp.videoPath))
					}

					// Set metadata path
					if dp.metaPath != "" {
						pe.metaPath = dp.metaPath
						pe.metaPathLabel.SetText(filepath.Base(dp.metaPath))
					}

					// Set source GoPro path
					if dp.sourcePath != "" {
						pe.sourcePath = dp.sourcePath
						pe.sourcePathLabel.SetText(filepath.Base(dp.sourcePath))
					}
				}
			}

			// Update status
			if len(detected) > 0 {
				statusLabel.SetText(fmt.Sprintf("Found %d MOV files. Auto-filled %d periods.",
					len(detected), min(len(detected), len(periodEntries))))
			} else {
				statusLabel.SetText("No MOV files found in folder.")
			}
		}, a.window)
	})

	addPeriodBtn := widget.NewButton("Add Period", func() {
		addPeriodCard()
	})

	removePeriodBtn := widget.NewButton("Remove Last Period", func() {
		if len(periodEntries) > 1 {
			periodEntries = periodEntries[:len(periodEntries)-1]
			periodsContainer.Remove(periodsContainer.Objects[len(periodsContainer.Objects)-1])
		}
	})

	analyzeBtn := widget.NewButton("Analyze Periods", func() {
		// Validate and collect periods
		var periods []metadata.Period
		for i, pe := range periodEntries {
			if pe.nameEntry.Text == "" {
				a.showError("Validation Error", fmt.Sprintf("Period %d: Name is required", i+1))
				return
			}
			if pe.videoPath == "" {
				a.showError("Validation Error", fmt.Sprintf("Period %d: Converted video is required", i+1))
				return
			}
			if pe.metaPath == "" {
				a.showError("Validation Error", fmt.Sprintf("Period %d: Metadata file is required", i+1))
				return
			}
			if pe.sourcePath == "" {
				a.showError("Validation Error", fmt.Sprintf("Period %d: Original GoPro file is required (for timecode)", i+1))
				return
			}

			periods = append(periods, metadata.Period{
				Name:         pe.nameEntry.Text,
				VideoFile:    pe.videoPath,
				MetadataFile: pe.metaPath,
				SourceGoPro:  pe.sourcePath,
			})
		}

		statusLabel.SetText("Analyzing periods...")

		go func() {
			analyzer := metadata.NewAnalyzer(a.ff)
			result, err := analyzer.AnalyzePeriods(periods)
			if err != nil {
				errMsg := "Error: " + err.Error()
				fyne.Do(func() {
					statusLabel.SetText(errMsg)
				})
				return
			}

			a.analysisResult = result
			a.periods = periods
			a.cfg.Periods = periods
			a.cfg.Save()

			// Display chapter summary
			var summary string
			summary = fmt.Sprintf("Found %d chapters across %d periods:\n\n", len(result.Chapters), len(periods))
			for _, ch := range result.Chapters {
				summary += fmt.Sprintf("%03d. [%s] %s Ch%d @ %s\n",
					ch.GlobalOrder,
					ch.Period,
					ch.ClockTime.Format("15:04:05"),
					ch.Number,
					metadata.FormatVideoTime(ch.VideoTime),
				)
			}

			fyne.Do(func() {
				statusLabel.SetText("Analysis complete!")
				chaptersLabel.SetText(summary)
				// Mark step complete
				a.markStepComplete(1)
			})
		}()
	})

	buttonRow := container.NewHBox(addPeriodBtn, removePeriodBtn, analyzeBtn)

	scroll := container.NewScroll(periodsContainer)

	helpText := widget.NewLabel("Select a folder to auto-detect files, or manually select each file.\n" +
		"Files are matched by base name: GX01.MOV ↔ GX01.MP4 ↔ GX01_metadata.txt")
	helpText.Wrapping = fyne.TextWrapWord

	folderRow := container.NewHBox(
		selectFolderBtn,
		folderLabel,
	)

	// Suppress unused variable warning
	_ = selectedFolder

	// Header section (fixed at top)
	header := container.NewVBox(
		widget.NewLabel("Step 2: Configure Periods"),
		widget.NewSeparator(),
		helpText,
		folderRow,
		widget.NewSeparator(),
	)

	// Status section with chapters results (scrollable)
	chaptersScroll := container.NewScroll(chaptersLabel)
	chaptersScroll.SetMinSize(fyne.NewSize(0, 100))

	// Footer section (fixed at bottom)
	footer := container.NewVBox(
		buttonRow,
		widget.NewSeparator(),
		statusLabel,
		chaptersScroll,
	)

	// Use Border layout so scroll area expands to fill available space
	return container.NewBorder(header, footer, nil, nil, scroll)
}
