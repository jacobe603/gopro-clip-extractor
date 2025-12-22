package ui

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// createStep1 creates the metadata extraction UI
func (a *App) createStep1() fyne.CanvasObject {
	// List of selected GoPro files
	selectedFiles := []string{}
	fileListLabel := widget.NewLabel("No files selected")
	fileListLabel.Wrapping = fyne.TextWrapWord

	// Status/progress area
	statusLabel := widget.NewLabel("")
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Results area
	resultsLabel := widget.NewLabel("")
	resultsLabel.Wrapping = fyne.TextWrapWord

	// Scroll container for file list (declare early so updateFileList can refresh it)
	fileListScroll := container.NewScroll(fileListLabel)
	fileListScroll.SetMinSize(fyne.NewSize(0, 100))

	updateFileList := func() {
		if len(selectedFiles) == 0 {
			fileListLabel.SetText("No files selected")
		} else {
			var names []string
			for _, f := range selectedFiles {
				names = append(names, filepath.Base(f))
			}
			fileListLabel.SetText(strings.Join(names, "\n"))
		}
		fileListLabel.Refresh()
		fileListScroll.Refresh()
	}

	// Select files button
	selectBtn := widget.NewButton("Select GoPro Files (.MP4)", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				a.showError("Error", err.Error())
				return
			}
			if reader == nil {
				return // User cancelled
			}
			reader.Close()

			path := reader.URI().Path()
			// On Windows, remove leading slash from /C:/...
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}

			// Check if already in list
			for _, f := range selectedFiles {
				if f == path {
					return
				}
			}

			selectedFiles = append(selectedFiles, path)
			updateFileList()

			// Save working directory
			a.cfg.LastWorkingDir = filepath.Dir(path)
		}, a.window)

		fd.SetFilter(storage.NewExtensionFileFilter([]string{".mp4", ".MP4"}))

		// Set initial directory if available
		if a.cfg.LastWorkingDir != "" {
			uri := storage.NewFileURI(a.cfg.LastWorkingDir)
			listable, err := storage.ListerForURI(uri)
			if err == nil {
				fd.SetLocation(listable)
			}
		}

		fd.Show()
	})

	// Add more files button
	addMoreBtn := widget.NewButton("Add More Files", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				a.showError("Error", err.Error())
				return
			}
			if reader == nil {
				return
			}
			reader.Close()

			path := reader.URI().Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}

			for _, f := range selectedFiles {
				if f == path {
					return
				}
			}

			selectedFiles = append(selectedFiles, path)
			updateFileList()
		}, a.window)

		fd.SetFilter(storage.NewExtensionFileFilter([]string{".mp4", ".MP4"}))
		if a.cfg.LastWorkingDir != "" {
			uri := storage.NewFileURI(a.cfg.LastWorkingDir)
			listable, err := storage.ListerForURI(uri)
			if err == nil {
				fd.SetLocation(listable)
			}
		}
		fd.Show()
	})

	// Clear button
	clearBtn := widget.NewButton("Clear All", func() {
		selectedFiles = []string{}
		updateFileList()
		resultsLabel.SetText("")
	})

	// Extract button
	extractBtn := widget.NewButton("Extract Metadata", func() {
		if len(selectedFiles) == 0 {
			a.showError("No Files", "Please select at least one GoPro file")
			return
		}

		progressBar.Show()
		progressBar.SetValue(0)
		statusLabel.SetText("Extracting metadata...")
		a.extractedMetadataFiles = []string{}

		go func() {
			var results []string
			for i, file := range selectedFiles {
				// Update progress (must use fyne.Do for thread safety)
				progress := float64(i+1) / float64(len(selectedFiles))
				fileName := filepath.Base(file)
				fyne.Do(func() {
					progressBar.SetValue(progress)
					statusLabel.SetText("Processing: " + fileName)
				})

				// Generate output path
				dir := filepath.Dir(file)
				base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
				outputPath := filepath.Join(dir, base+"_metadata.txt")

				// Extract metadata
				err := a.ff.ExtractMetadata(file, outputPath)
				if err != nil {
					results = append(results, "FAILED: "+filepath.Base(file)+" - "+err.Error())
				} else {
					results = append(results, "OK: "+filepath.Base(file)+" -> "+filepath.Base(outputPath))
					a.extractedMetadataFiles = append(a.extractedMetadataFiles, outputPath)
				}
			}

			finalResults := "Results:\n" + strings.Join(results, "\n")
			fyne.Do(func() {
				progressBar.Hide()
				statusLabel.SetText("Done!")
				resultsLabel.SetText(finalResults)
				// Mark step complete if we extracted at least one file
				if len(a.extractedMetadataFiles) > 0 {
					a.markStepComplete(0)
				}
			})
		}()
	})

	// Layout
	buttonsRow := container.NewHBox(selectBtn, addMoreBtn, clearBtn)
	extractRow := container.NewHBox(extractBtn)

	return container.NewVBox(
		widget.NewLabel("Step 1: Extract Metadata from GoPro Files"),
		widget.NewSeparator(),
		widget.NewLabel("Select your original GoPro source files (GX*.MP4):"),
		buttonsRow,
		widget.NewLabel("Selected files:"),
		fileListScroll,
		widget.NewSeparator(),
		extractRow,
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		resultsLabel,
	)
}
