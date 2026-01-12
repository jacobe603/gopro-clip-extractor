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

	// Status
	statusLabel := widget.NewLabel("")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

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
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".mp4") {
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

		// Sort clips by filename
		sort.Strings(toCombine)

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
			finalOutput = filepath.Join(outputDir, fmt.Sprintf("combined_%s.mp4", timestamp))
		}

		progressBar.Show()
		progressBar.SetValue(0)
		statusLabel.SetText("Combining clips...")

		go func() {
			fyne.Do(func() {
				progressBar.SetValue(0.5) // Indeterminate-ish
			})

			err := a.ff.ConcatClips(toCombine, finalOutput)
			if err != nil {
				errMsg := "Error: " + err.Error()
				fyne.Do(func() {
					progressBar.Hide()
					statusLabel.SetText(errMsg)
				})
				return
			}

			successMsg := fmt.Sprintf("Done! Combined %d clips into:\n%s", len(toCombine), finalOutput)
			fyne.Do(func() {
				progressBar.SetValue(1.0)
				progressBar.Hide()
				statusLabel.SetText(successMsg)
				// Mark step complete
				a.markStepComplete(3)
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

	selectionBtns := container.NewHBox(selectAllBtn, deselectAllBtn)

	scroll := container.NewScroll(clipsContainer)
	scroll.SetMinSize(fyne.NewSize(0, 300))

	return container.NewVBox(
		widget.NewLabel("Step 4: Combine Clips into Highlight Reel"),
		widget.NewSeparator(),
		inputRow,
		outputRow,
		widget.NewSeparator(),
		widget.NewLabel("Select clips to combine (in order):"),
		selectionBtns,
		scroll,
		widget.NewSeparator(),
		combineBtn,
		statusLabel,
		progressBar,
	)
}
