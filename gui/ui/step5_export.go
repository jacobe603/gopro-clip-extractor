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

// createStep5Export creates the full game export UI
func (a *App) createStep5Export() fyne.CanvasObject {
	// MOV files list
	var movFiles []string
	movListLabel := widget.NewLabel("No MOV files detected")
	movListLabel.Wrapping = fyne.TextWrapWord

	// Output file
	outputFileLabel := widget.NewLabel("(auto-generated)")
	var outputFile string

	// Status
	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Quality preset
	qualitySelect := widget.NewSelect([]string{
		"High Quality (CRF 18) - ~12 Mbps",
		"Balanced (CRF 20) - ~8 Mbps",
		"Smaller File (CRF 23) - ~5 Mbps",
	}, nil)
	qualitySelect.SetSelected("Balanced (CRF 20) - ~8 Mbps")

	// Refresh MOV files from working folder
	refreshMOVs := func() {
		movFiles = nil
		if a.workingFolder == "" {
			movListLabel.SetText("No working folder set. Complete Step 1 first.")
			return
		}

		entries, err := os.ReadDir(a.workingFolder)
		if err != nil {
			movListLabel.SetText("Error reading folder: " + err.Error())
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".mov" {
				movFiles = append(movFiles, filepath.Join(a.workingFolder, entry.Name()))
			}
		}

		sort.Strings(movFiles)

		if len(movFiles) == 0 {
			movListLabel.SetText("No MOV files found in working folder.")
			return
		}

		// Build display text with durations
		var lines []string
		var totalDuration float64
		for _, f := range movFiles {
			dur, _ := a.ff.GetDuration(f)
			totalDuration += dur
			durStr := formatDuration(dur)
			lines = append(lines, fmt.Sprintf("  %s (%s)", filepath.Base(f), durStr))
		}
		lines = append(lines, fmt.Sprintf("\nTotal: %s", formatDuration(totalDuration)))
		movListLabel.SetText(strings.Join(lines, "\n"))
	}

	refreshBtn := widget.NewButton("Refresh", func() {
		refreshMOVs()
	})

	selectFolderBtn := widget.NewButton("Select Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			a.workingFolder = path
			refreshMOVs()
		}, a.window)
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
			// Ensure .mp4 extension
			if !strings.HasSuffix(strings.ToLower(path), ".mp4") {
				path = path + ".mp4"
			}
			outputFile = path
			outputFileLabel.SetText(filepath.Base(path))
		}, a.window)
	})

	exportBtn := widget.NewButton("Export Full Game", func() {
		if len(movFiles) == 0 {
			a.showError("No Files", "No MOV files to export. Click Refresh or select a folder.")
			return
		}

		// Generate output filename if not set
		finalOutput := outputFile
		if finalOutput == "" {
			timestamp := time.Now().Format("2006-01-02")
			finalOutput = filepath.Join(a.workingFolder, fmt.Sprintf("FullGame_%s.mp4", timestamp))
		}

		// Parse quality setting
		crf := "20" // default balanced
		switch qualitySelect.Selected {
		case "High Quality (CRF 18) - ~12 Mbps":
			crf = "18"
		case "Balanced (CRF 20) - ~8 Mbps":
			crf = "20"
		case "Smaller File (CRF 23) - ~5 Mbps":
			crf = "23"
		}

		progressBar.Show()
		progressBar.SetValue(0)
		statusLabel.SetText("Exporting full game video...")

		go func() {
			// Export with chapter preservation
			err := a.ff.ExportFullGame(movFiles, finalOutput, crf, func(progress float64, status string) {
				fyne.Do(func() {
					progressBar.SetValue(progress)
					statusLabel.SetText(status)
				})
			})

			fyne.Do(func() {
				progressBar.Hide()
				if err != nil {
					statusLabel.SetText("Error: " + err.Error())
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
					statusLabel.SetText(fmt.Sprintf("Done! Exported to:\n%s\nSize: %s", finalOutput, sizeStr))
					a.markStepComplete(4)
				}
			})
		}()
	})

	// Initial refresh
	refreshMOVs()

	// Layout
	helpText := widget.NewLabel("Combine period MOV files into a single YouTube-ready video.\n" +
		"Preserves chapter markers (HiLight tags) with correct timestamps.\n" +
		"Re-encodes to H.264 High profile with AAC audio.")
	helpText.Wrapping = fyne.TextWrapWord

	header := container.NewVBox(
		widget.NewLabel("Step 5: Export Full Game"),
		widget.NewSeparator(),
		helpText,
		widget.NewSeparator(),
	)

	filesSection := container.NewVBox(
		widget.NewLabel("Source MOV Files:"),
		container.NewHBox(refreshBtn, selectFolderBtn),
		movListLabel,
		widget.NewSeparator(),
	)

	outputSection := container.NewVBox(
		container.NewHBox(
			widget.NewLabel("Output:"),
			outputFileLabel,
			selectOutputBtn,
		),
		container.NewHBox(
			widget.NewLabel("Quality:"),
			qualitySelect,
		),
		widget.NewSeparator(),
	)

	footer := container.NewVBox(
		exportBtn,
		progressBar,
		statusLabel,
	)

	return container.NewVBox(
		header,
		filesSection,
		outputSection,
		footer,
	)
}

// formatDuration formats seconds as HH:MM:SS
func formatDuration(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
