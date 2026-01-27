package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// createStep4Combine creates the combine clips UI
func (a *App) createStep4Combine() fyne.CanvasObject {
	// Clips list
	selectedClips := make(map[string]bool)
	var checkboxes []*widget.Check
	clipsContainer := container.NewVBox()

	// Input/Output
	inputFolderLabel := widget.NewLabel("(none selected)")
	var inputFolder string
	outputFileLabel := widget.NewLabel("(auto-generated)")
	var outputFile string

	// Status and progress
	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	elapsedLabel := widget.NewLabel("")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Timer and cancel control
	var timerStop chan bool
	var combineRunning bool

	cancelBtn := widget.NewButton("Cancel", nil)
	cancelBtn.Hide()

	// Encoding options
	reencodeCheck := widget.NewCheck("Re-encode (smaller file, slower)", nil)
	reencodeCheck.SetChecked(false)

	qualitySelect := widget.NewSelect([]string{
		"High Quality (CRF 18) - ~12 Mbps",
		"Balanced (CRF 20) - ~8 Mbps",
		"Smaller File (CRF 23) - ~5 Mbps",
		"Smallest (CPU, CRF 23) - best compression",
	}, nil)
	qualitySelect.SetSelected("Smaller File (CRF 23) - ~5 Mbps")
	qualitySelect.Disable() // Disabled until re-encode is checked

	reencodeCheck.OnChanged = func(checked bool) {
		if checked {
			qualitySelect.Enable()
		} else {
			qualitySelect.Disable()
		}
	}

	// Refresh clips list from folder
	refreshClips := func() {
		clipsContainer.Objects = nil
		checkboxes = nil
		selectedClips = make(map[string]bool)

		if inputFolder == "" {
			// Try to use extracted clips from step 2
			if len(a.extractedClips) > 0 {
				for _, clip := range a.extractedClips {
					clip := clip
					check := widget.NewCheck(filepath.Base(clip), func(checked bool) {
						selectedClips[clip] = checked
					})
					check.SetChecked(true)
					selectedClips[clip] = true
					checkboxes = append(checkboxes, check)
					clipsContainer.Add(check)
				}
				clipsContainer.Refresh()
				return
			}

			clipsContainer.Add(widget.NewLabel("No clips available. Either select an input folder or extract clips in Step 2."))
			clipsContainer.Refresh()
			return
		}

		// Scan input folder for MP4 files
		entries, err := os.ReadDir(inputFolder)
		if err != nil {
			clipsContainer.Add(widget.NewLabel("Error reading folder: " + err.Error()))
			clipsContainer.Refresh()
			return
		}

		var clips []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".mp4" || ext == ".mov" {
				clips = append(clips, filepath.Join(inputFolder, entry.Name()))
			}
		}

		// Sort by filename (which should be chronological with our naming scheme)
		sort.Strings(clips)

		if len(clips) == 0 {
			clipsContainer.Add(widget.NewLabel("No MP4 files found in folder"))
			clipsContainer.Refresh()
			return
		}

		for _, clip := range clips {
			clip := clip
			check := widget.NewCheck(filepath.Base(clip), func(checked bool) {
				selectedClips[clip] = checked
			})
			check.SetChecked(true)
			selectedClips[clip] = true
			checkboxes = append(checkboxes, check)
			clipsContainer.Add(check)
		}
		clipsContainer.Refresh()
	}

	selectInputBtn := widget.NewButton("Select Input Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			inputFolder = path
			inputFolderLabel.SetText(path)
			refreshClips()
		}, a.window)
	})

	useStep2Btn := widget.NewButton("Use Clips from Step 2", func() {
		// Use the output folder from Step 2 (where clips were extracted to)
		if a.cfg.LastOutputDir != "" {
			inputFolder = a.cfg.LastOutputDir
			inputFolderLabel.SetText(a.cfg.LastOutputDir)
			refreshClips()
		} else if len(a.extractedClips) > 0 {
			// Fall back to in-memory list if no output dir saved
			inputFolder = ""
			inputFolderLabel.SetText("(using session clips)")
			refreshClips()
		} else {
			statusLabel.SetText("No output folder from Step 2. Please select a folder.")
		}
	})

	selectOutputBtn := widget.NewButton("Select Output File", func() {
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			writer.Close()
			path := writer.URI().Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			outputFile = path
			outputFileLabel.SetText(filepath.Base(path))
		}, a.window)
	})

	selectAllBtn := widget.NewButton("Select All", func() {
		for _, cb := range checkboxes {
			cb.SetChecked(true)
		}
		for clip := range selectedClips {
			selectedClips[clip] = true
		}
	})

	deselectAllBtn := widget.NewButton("Deselect All", func() {
		for _, cb := range checkboxes {
			cb.SetChecked(false)
		}
		for clip := range selectedClips {
			selectedClips[clip] = false
		}
	})

	// Cancel button handler
	cancelBtn.OnTapped = func() {
		if combineRunning {
			statusLabel.SetText("Cancelling...")
			a.ff.CancelExport()
		}
	}

	combineBtn := widget.NewButton("Combine Clips", func() {
		// Get selected clips
		var toCombine []string
		for clip, selected := range selectedClips {
			if selected {
				toCombine = append(toCombine, clip)
			}
		}

		if len(toCombine) == 0 {
			a.showError("No Clips", "Please select at least one clip to combine")
			return
		}

		if combineRunning {
			return // Already running
		}

		// Sort clips by filename
		sort.Strings(toCombine)

		// Parse encoding settings first (needed for output extension)
		useReencode := reencodeCheck.Checked

		// Generate output filename if not set
		finalOutput := outputFile
		if finalOutput == "" {
			var outputDir string
			if inputFolder != "" {
				outputDir = filepath.Dir(inputFolder)
			} else if len(toCombine) > 0 {
				outputDir = filepath.Dir(toCombine[0])
			} else {
				outputDir = "."
			}
			timestamp := time.Now().Format("2006-01-02_15-04")

			// Determine output extension:
			// - Re-encode always outputs MP4
			// - Stream copy uses same extension as input files
			ext := ".mp4"
			if !useReencode && len(toCombine) > 0 {
				ext = strings.ToLower(filepath.Ext(toCombine[0]))
				if ext != ".mp4" && ext != ".mov" {
					ext = ".mp4" // fallback
				}
			}
			finalOutput = filepath.Join(outputDir, fmt.Sprintf("combined_%s%s", timestamp, ext))
		}
		crf := "23"
		forceCPU := false
		encoderName := "Stream Copy"

		if useReencode {
			switch qualitySelect.Selected {
			case "High Quality (CRF 18) - ~12 Mbps":
				crf = "18"
				encoderName = "GPU (CRF 18)"
			case "Balanced (CRF 20) - ~8 Mbps":
				crf = "20"
				encoderName = "GPU (CRF 20)"
			case "Smaller File (CRF 23) - ~5 Mbps":
				crf = "23"
				encoderName = "GPU (CRF 23)"
			case "Smallest (CPU, CRF 23) - best compression":
				crf = "23"
				forceCPU = true
				encoderName = "CPU (CRF 23)"
			}
		}

		// Reset cancel state
		a.ff.ResetCancel()
		combineRunning = true

		progressBar.Show()
		progressBar.SetValue(0)
		elapsedLabel.SetText("")

		if useReencode {
			cancelBtn.Show()
			statusLabel.SetText(fmt.Sprintf("Combining %d clips with %s encoding...", len(toCombine), encoderName))
		} else {
			statusLabel.SetText(fmt.Sprintf("Combining %d clips (stream copy)...", len(toCombine)))
		}

		// Start elapsed time timer for re-encode
		startTime := time.Now()
		timerStop = make(chan bool, 1)

		if useReencode {
			go func() {
				ticker := time.NewTicker(1 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						elapsed := time.Since(startTime)
						fyne.Do(func() {
							elapsedLabel.SetText(fmt.Sprintf("Elapsed: %s", formatDuration(elapsed.Seconds())))
						})
					case <-timerStop:
						return
					}
				}
			}()
		}

		go func() {
			fyne.Do(func() {
				if !useReencode {
					progressBar.SetValue(0.5) // Indeterminate for stream copy
				} else {
					progressBar.SetValue(0.15) // Show we're encoding
				}
			})

			var err error
			if useReencode {
				err = a.ff.ConcatClipsWithEncode(toCombine, finalOutput, crf, forceCPU)
			} else {
				err = a.ff.ConcatClips(toCombine, finalOutput)
			}

			// Stop the timer
			close(timerStop)
			combineRunning = false
			totalElapsed := time.Since(startTime)

			fyne.Do(func() {
				progressBar.SetValue(1.0)
				progressBar.Hide()
				cancelBtn.Hide()

				if err != nil {
					if a.ff.IsCancelled() {
						elapsedLabel.SetText(fmt.Sprintf("Cancelled after %s", formatDuration(totalElapsed.Seconds())))
						statusLabel.SetText("Combine cancelled.")
						os.Remove(finalOutput)
					} else {
						elapsedLabel.SetText("")
						statusLabel.SetText("Error: " + err.Error())
					}
				} else {
					// Get final file size
					info, _ := os.Stat(finalOutput)
					var sizeStr string
					if info != nil {
						sizeMB := float64(info.Size()) / (1024 * 1024)
						if sizeMB > 1024 {
							sizeStr = fmt.Sprintf("%.1f GB", sizeMB/1024)
						} else {
							sizeStr = fmt.Sprintf("%.0f MB", sizeMB)
						}
					}

					if useReencode {
						elapsedLabel.SetText(fmt.Sprintf("Completed in %s", formatDuration(totalElapsed.Seconds())))
					} else {
						elapsedLabel.SetText("")
					}
					statusLabel.SetText(fmt.Sprintf("Done! Combined %d clips into:\n%s\nSize: %s", len(toCombine), finalOutput, sizeStr))
					a.markStepComplete(3)
				}
			})
		}()
	})

	// Initial refresh
	refreshClips()

	// Layout
	inputRow := container.NewHBox(
		widget.NewLabel("Input:"),
		inputFolderLabel,
		selectInputBtn,
		useStep2Btn,
	)

	outputRow := container.NewHBox(
		widget.NewLabel("Output:"),
		outputFileLabel,
		selectOutputBtn,
	)

	encodingRow := container.NewVBox(
		reencodeCheck,
		container.NewHBox(widget.NewLabel("  Quality:"), qualitySelect),
	)

	selectionBtns := container.NewHBox(selectAllBtn, deselectAllBtn)

	scroll := container.NewScroll(clipsContainer)
	scroll.SetMinSize(fyne.NewSize(0, 250))

	return container.NewVBox(
		widget.NewLabel("Step 4: Combine Clips into Highlight Reel"),
		widget.NewSeparator(),
		inputRow,
		outputRow,
		encodingRow,
		widget.NewSeparator(),
		widget.NewLabel("Select clips to combine (in order):"),
		selectionBtns,
		scroll,
		widget.NewSeparator(),
		container.NewHBox(combineBtn, cancelBtn),
		progressBar,
		elapsedLabel,
		statusLabel,
	)
}
