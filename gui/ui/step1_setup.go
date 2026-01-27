package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"gopro-gui/metadata"
)

// detectedFile holds information about a detected file
type detectedFile struct {
	path         string
	baseName     string
	fileType     string // "mov", "mp4", "metadata"
	hasTimecode  bool
	timecode     string
	hasChapters  bool
	chapterCount int
}

// detectedPeriodInfo holds auto-detected period information
type detectedPeriodInfo struct {
	name           string
	movFile        *detectedFile
	mp4File        *detectedFile
	metadataFile   *detectedFile
	metadataSource string // "mov", "metadata", "needs_extraction"
	ready          bool
}

// splitGoProGroup holds information about split GoPro files
type splitGoProGroup struct {
	videoID  string   // e.g., "0092"
	prefix   string   // e.g., "GX" or "GH"
	fileType string   // "MP4" or "MOV"
	files    []string // sorted by sequence: [GX010092.MP4, GX020092.MP4]
	combined bool     // true if already combined
}

// detectSplitGoProFiles scans a folder for split GoPro file groups
// Returns groups that have 2+ files with the same video ID
// Detects both original MP4 files and converted MOV files
func detectSplitGoProFiles(folderPath string) ([]splitGoProGroup, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}

	// Pattern: GX##XXXX.MP4/mp4/MOV/mov or GH##XXXX.MP4/mp4/MOV/mov
	// Group 1: prefix (GX/GH)
	// Group 2: sequence number (01, 02, ...)
	// Group 3: video ID (0092, etc.)
	// Group 4: extension
	pattern := regexp.MustCompile(`^(GX|GH)(\d{2})(\d{4})\.(MP4|mp4|MOV|mov)$`)

	// Map: prefix+videoID+ext -> list of (sequence, filename)
	type fileInfo struct {
		sequence string
		filename string
		fullPath string
	}
	groups := make(map[string][]fileInfo)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		matches := pattern.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		prefix := matches[1]
		sequence := matches[2]
		videoID := matches[3]
		ext := strings.ToUpper(matches[4])
		key := prefix + "_" + videoID + "_" + ext

		groups[key] = append(groups[key], fileInfo{
			sequence: sequence,
			filename: name,
			fullPath: filepath.Join(folderPath, name),
		})
	}

	// Build result: only groups with 2+ files
	var result []splitGoProGroup
	for key, files := range groups {
		if len(files) < 2 {
			continue
		}

		// Sort by sequence
		sort.Slice(files, func(i, j int) bool {
			return files[i].sequence < files[j].sequence
		})

		parts := strings.Split(key, "_")
		prefix := parts[0]
		videoID := parts[1]
		fileType := parts[2]

		var filePaths []string
		for _, f := range files {
			filePaths = append(filePaths, f.fullPath)
		}

		result = append(result, splitGoProGroup{
			videoID:  videoID,
			prefix:   prefix,
			fileType: fileType,
			files:    filePaths,
		})
	}

	// Sort by video ID
	sort.Slice(result, func(i, j int) bool {
		return result[i].videoID < result[j].videoID
	})

	return result, nil
}

// createStep1Setup creates the setup/folder detection UI
func (a *App) createStep1Setup() fyne.CanvasObject {
	var detectedPeriods []*detectedPeriodInfo
	var workingFolder string
	var splitGroups []splitGoProGroup
	var splitCheckboxes []*widget.Check

	// UI elements
	folderLabel := widget.NewLabel("No folder selected")
	folderLabel.Wrapping = fyne.TextWrapWord

	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord

	filesFoundLabel := widget.NewLabel("")
	filesFoundLabel.Wrapping = fyne.TextWrapWord

	periodsContainer := container.NewVBox()
	periodsScroll := container.NewScroll(periodsContainer)
	periodsScroll.SetMinSize(fyne.NewSize(0, 250))

	// Split GoPro files UI elements
	splitContainer := container.NewVBox()
	splitSection := container.NewVBox()
	splitSection.Hide()

	combineBtn := widget.NewButton("Combine Selected", nil)
	combineBtn.Disable()

	combineProgressBar := widget.NewProgressBar()
	combineProgressBar.Hide()

	extractBtn := widget.NewButton("Extract Metadata from GoPro Files", nil)
	extractBtn.Hide()

	scanProgressBar := widget.NewProgressBar()
	scanProgressBar.Hide()

	extractProgressBar := widget.NewProgressBar()
	extractProgressBar.Hide()

	analyzeBtn := widget.NewButton("Analyze & Continue", nil)
	analyzeBtn.Disable()

	// Scan and categorize folder (runs in background)
	scanFolder := func(folderPath string) {
		// Clear previous results and show scanning indicator
		detectedPeriods = nil
		splitGroups = nil
		splitCheckboxes = nil
		periodsContainer.Objects = nil
		periodsContainer.Refresh()
		splitContainer.Objects = nil
		splitContainer.Refresh()
		splitSection.Hide()
		filesFoundLabel.SetText("")
		statusLabel.SetText("Scanning folder...")
		scanProgressBar.SetValue(0)
		scanProgressBar.Show()
		analyzeBtn.Disable()
		extractBtn.Hide()
		combineBtn.Disable()

		go func() {
			entries, err := os.ReadDir(folderPath)
			if err != nil {
				fyne.Do(func() {
					statusLabel.SetText("Error reading folder: " + err.Error())
					scanProgressBar.Hide()
				})
				return
			}

			// Detect split GoPro files
			detectedSplitGroups, _ := detectSplitGoProFiles(folderPath)

			// First pass: count video files and collect metadata files (fast)
			var videoFiles []struct {
				path     string
				baseName string
				ext      string
			}
			var metaFiles []detectedFile

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				ext := strings.ToLower(filepath.Ext(name))
				baseName := strings.TrimSuffix(name, filepath.Ext(name))
				fullPath := filepath.Join(folderPath, name)

				switch ext {
				case ".mov", ".mp4":
					if ext == ".mp4" && strings.HasSuffix(baseName, "_metadata") {
						continue
					}
					videoFiles = append(videoFiles, struct {
						path     string
						baseName string
						ext      string
					}{fullPath, baseName, ext})
				case ".txt":
					if strings.HasSuffix(baseName, "_metadata") {
						realBaseName := strings.TrimSuffix(baseName, "_metadata")
						metaFiles = append(metaFiles, detectedFile{
							path:     fullPath,
							baseName: realBaseName,
							fileType: "metadata",
						})
					}
				}
			}

			totalFiles := len(videoFiles)
			if totalFiles == 0 {
				fyne.Do(func() {
					statusLabel.SetText("No video files found.")
					scanProgressBar.Hide()
				})
				return
			}

			// Second pass: check metadata for each video file (slow)
			var movFiles, mp4Files []detectedFile
			for i, vf := range videoFiles {
				fyne.Do(func() {
					scanProgressBar.SetValue(float64(i) / float64(totalFiles))
					statusLabel.SetText(fmt.Sprintf("Scanning %d/%d: %s...", i+1, totalFiles, filepath.Base(vf.path)))
				})

				df := detectedFile{
					path:     vf.path,
					baseName: vf.baseName,
					fileType: strings.TrimPrefix(vf.ext, "."),
				}

				// Check for metadata (this is the slow part)
				info, err := a.ff.CheckVideoMetadata(vf.path)
				if err == nil {
					df.hasTimecode = info.HasTimecode
					df.timecode = info.Timecode
					df.hasChapters = info.HasChapters
					df.chapterCount = info.ChapterCount
				}

				if vf.ext == ".mov" {
					movFiles = append(movFiles, df)
				} else {
					mp4Files = append(mp4Files, df)
				}
			}

			// Sort by base name
			sort.Slice(movFiles, func(i, j int) bool { return movFiles[i].baseName < movFiles[j].baseName })
			sort.Slice(mp4Files, func(i, j int) bool { return mp4Files[i].baseName < mp4Files[j].baseName })

			// Update UI with results
			fyne.Do(func() {
				scanProgressBar.SetValue(1.0)
				filesFoundLabel.SetText(fmt.Sprintf("Found: %d MOV files, %d MP4 files, %d metadata files",
					len(movFiles), len(mp4Files), len(metaFiles)))

				// Show split GoPro files section if any detected
				splitGroups = detectedSplitGroups
				splitCheckboxes = nil
				splitContainer.Objects = nil

				if len(splitGroups) > 0 {
					splitContainer.Add(widget.NewLabel(fmt.Sprintf("Found %d split video groups:", len(splitGroups))))

					for i := range splitGroups {
						group := &splitGroups[i]
						// Build file list string
						var fileNames []string
						for _, f := range group.files {
							fileNames = append(fileNames, filepath.Base(f))
						}
						filesStr := strings.Join(fileNames, ", ")

						checkText := fmt.Sprintf("[%s] Video %s (%d parts: %s)", group.fileType, group.videoID, len(group.files), filesStr)
						check := widget.NewCheck(checkText, func(checked bool) {
							// Update combine button state
							anyChecked := false
							for _, cb := range splitCheckboxes {
								if cb.Checked {
									anyChecked = true
									break
								}
							}
							if anyChecked {
								combineBtn.Enable()
							} else {
								combineBtn.Disable()
							}
						})
						check.SetChecked(true)
						splitCheckboxes = append(splitCheckboxes, check)
						splitContainer.Add(check)
					}

					combineBtn.Enable()
					splitSection.Show()
				} else {
					splitSection.Hide()
				}
				splitContainer.Refresh()

				if len(movFiles) == 0 {
					statusLabel.SetText("No MOV files found. Please select a folder with converted video files.")
					scanProgressBar.Hide()
					return
				}

				// Auto-create periods based on MOV files
				needsExtraction := false
				allReady := true

				for i := range movFiles {
					mov := &movFiles[i]
					period := &detectedPeriodInfo{
						name:    fmt.Sprintf("Period %d", i+1),
						movFile: mov,
					}

					// Find matching MP4 by base name
					for j := range mp4Files {
						if mp4Files[j].baseName == mov.baseName {
							period.mp4File = &mp4Files[j]
							break
						}
					}

					// Find matching metadata file
					for j := range metaFiles {
						if metaFiles[j].baseName == mov.baseName {
							period.metadataFile = &metaFiles[j]
							break
						}
					}

					// Determine metadata source
					if mov.hasChapters && mov.hasTimecode {
						period.metadataSource = "mov"
						period.ready = true
					} else if period.metadataFile != nil {
						if mov.hasTimecode || (period.mp4File != nil && period.mp4File.hasTimecode) {
							period.metadataSource = "metadata"
							period.ready = true
						} else {
							period.metadataSource = "needs_extraction"
							period.ready = false
							allReady = false
						}
					} else if period.mp4File != nil && period.mp4File.hasChapters {
						period.metadataSource = "needs_extraction"
						period.ready = false
						needsExtraction = true
						allReady = false
					} else {
						period.metadataSource = "needs_extraction"
						period.ready = false
						allReady = false
					}

					detectedPeriods = append(detectedPeriods, period)

					// Create period card
					var statusText string
					switch period.metadataSource {
					case "mov":
						statusText = fmt.Sprintf("Timecode: %s, %d chapters (from MOV)", mov.timecode, mov.chapterCount)
					case "metadata":
						statusText = "Using _metadata.txt file"
					case "needs_extraction":
						if period.mp4File != nil && period.mp4File.hasChapters {
							statusText = fmt.Sprintf("Need to extract (%d chapters in GoPro file)", period.mp4File.chapterCount)
						} else {
							statusText = "No metadata available"
						}
					}

					cardContent := container.NewVBox(
						widget.NewLabel(fmt.Sprintf("Video: %s", filepath.Base(mov.path))),
						widget.NewLabel(fmt.Sprintf("Status: %s", statusText)),
					)

					if period.mp4File != nil {
						cardContent.Add(widget.NewLabel(fmt.Sprintf("GoPro source: %s", filepath.Base(period.mp4File.path))))
					}

					card := widget.NewCard(period.name, mov.baseName, cardContent)
					periodsContainer.Add(card)
				}

				periodsContainer.Refresh()
				scanProgressBar.Hide()

				// Update buttons based on status
				if needsExtraction {
					extractBtn.Show()
					statusLabel.SetText("Some MOV files are missing metadata. Click 'Extract Metadata' to extract from GoPro files.")
				} else {
					extractBtn.Hide()
				}

				if allReady {
					analyzeBtn.Enable()
					statusLabel.SetText(fmt.Sprintf("Ready! Found %d periods with metadata.", len(detectedPeriods)))
				} else if !needsExtraction {
					analyzeBtn.Disable()
					statusLabel.SetText("Some periods are missing required metadata.")
				}
			})
		}()
	}

	// Select folder button
	selectFolderBtn := widget.NewButton("Select Folder", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.Path()
			if len(path) > 2 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			workingFolder = path
			folderLabel.SetText(path)
			a.cfg.LastWorkingDir = path

			scanFolder(path)
		}, a.window)
	})

	// Refresh button
	refreshBtn := widget.NewButton("Refresh", func() {
		if workingFolder != "" {
			scanFolder(workingFolder)
		}
	})

	// Extract metadata button
	extractBtn.OnTapped = func() {
		if workingFolder == "" {
			return
		}

		extractBtn.Disable()
		extractProgressBar.Show()
		extractProgressBar.SetValue(0)

		go func() {
			var toExtract []*detectedPeriodInfo
			for _, p := range detectedPeriods {
				if p.metadataSource == "needs_extraction" && p.mp4File != nil {
					toExtract = append(toExtract, p)
				}
			}

			for i, p := range toExtract {
				fyne.Do(func() {
					extractProgressBar.SetValue(float64(i) / float64(len(toExtract)))
					statusLabel.SetText(fmt.Sprintf("Extracting %d/%d: %s...", i+1, len(toExtract), p.mp4File.baseName))
				})

				// Generate output path
				outputPath := filepath.Join(workingFolder, p.mp4File.baseName+"_metadata.txt")
				err := a.ff.ExtractMetadata(p.mp4File.path, outputPath)
				if err == nil {
					p.metadataFile = &detectedFile{
						path:     outputPath,
						baseName: p.mp4File.baseName,
						fileType: "metadata",
					}
					p.metadataSource = "metadata"
					p.ready = true
				}
			}

			fyne.Do(func() {
				extractProgressBar.SetValue(1.0)
				extractProgressBar.Hide()
				extractBtn.Enable()
				scanFolder(workingFolder) // Refresh the display
			})
		}()
	}

	// Combine split files button
	combineBtn.OnTapped = func() {
		if workingFolder == "" || len(splitGroups) == 0 {
			return
		}

		// Get selected groups
		var toCombine []splitGoProGroup
		for i, cb := range splitCheckboxes {
			if cb.Checked && i < len(splitGroups) {
				toCombine = append(toCombine, splitGroups[i])
			}
		}

		if len(toCombine) == 0 {
			return
		}

		combineBtn.Disable()
		combineProgressBar.Show()
		combineProgressBar.SetValue(0)

		go func() {
			for i, group := range toCombine {
				fyne.Do(func() {
					combineProgressBar.SetValue(float64(i) / float64(len(toCombine)))
					statusLabel.SetText(fmt.Sprintf("Combining %d/%d: %s Video %s (%d parts)...",
						i+1, len(toCombine), group.fileType, group.videoID, len(group.files)))
				})

				// Output filename: {prefix}_combined_{videoID}.{ext}
				outputPath := filepath.Join(workingFolder,
					fmt.Sprintf("%s_combined_%s.%s", group.prefix, group.videoID, group.fileType))

				err := a.ff.CombineSplitGoPro(group.files, outputPath)
				if err != nil {
					fyne.Do(func() {
						statusLabel.SetText(fmt.Sprintf("Error combining video %s: %s", group.videoID, err.Error()))
					})
				}
			}

			fyne.Do(func() {
				combineProgressBar.SetValue(1.0)
				combineProgressBar.Hide()
				combineBtn.Enable()
				statusLabel.SetText(fmt.Sprintf("Combined %d video groups. Rescanning folder...", len(toCombine)))
				scanFolder(workingFolder) // Refresh to show new combined files
			})
		}()
	}

	// Analyze & Continue button
	analyzeBtn.OnTapped = func() {
		if len(detectedPeriods) == 0 {
			return
		}

		analyzeBtn.Disable()
		statusLabel.SetText("Analyzing periods...")

		go func() {
			// Build periods for analysis
			var periods []metadata.Period
			for i, dp := range detectedPeriods {
				p := metadata.Period{
					Name:      fmt.Sprintf("%dPeriod", i+1),
					VideoFile: dp.movFile.path,
				}

				if dp.metadataSource == "mov" {
					p.UseMovMetadata = true
					p.MetadataFile = dp.movFile.path
					p.SourceGoPro = dp.movFile.path
				} else if dp.metadataFile != nil {
					p.MetadataFile = dp.metadataFile.path
					if dp.mp4File != nil {
						p.SourceGoPro = dp.mp4File.path
					} else {
						p.SourceGoPro = dp.movFile.path
						p.UseMovMetadata = true
					}
				}

				periods = append(periods, p)
			}

			// Run analysis
			analyzer := metadata.NewAnalyzer(a.ff)
			result, err := analyzer.AnalyzePeriods(periods)
			if err != nil {
				fyne.Do(func() {
					statusLabel.SetText("Error: " + err.Error())
					analyzeBtn.Enable()
				})
				return
			}

			a.analysisResult = result
			a.periods = periods
			a.workingFolder = workingFolder
			a.cfg.Periods = periods
			a.cfg.Save()

			fyne.Do(func() {
				statusLabel.SetText(fmt.Sprintf("Analysis complete! Found %d chapters across %d periods.",
					len(result.Chapters), len(periods)))
				a.markStepComplete(0)
				analyzeBtn.Enable()

				// Auto-switch to next tab
				a.tabs.SelectIndex(1)
			})
		}()
	}

	// Layout
	folderRow := container.NewBorder(nil, nil, widget.NewLabel("Working Folder:"), container.NewHBox(selectFolderBtn, refreshBtn), folderLabel)

	// Build split section UI
	splitSection.Objects = []fyne.CanvasObject{
		widget.NewSeparator(),
		widget.NewLabel("Split GoPro Files"),
		splitContainer,
		container.NewHBox(combineBtn, combineProgressBar),
	}

	header := container.NewVBox(
		widget.NewLabel("Step 1: Setup"),
		widget.NewSeparator(),
		widget.NewLabel("Select the folder containing your GoPro files (MOV + MP4):"),
		folderRow,
		widget.NewSeparator(),
		filesFoundLabel,
		splitSection,
		widget.NewSeparator(),
		widget.NewLabel("Detected Periods"),
	)

	extractRow := container.NewVBox(
		extractBtn,
		extractProgressBar,
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		extractRow,
		statusLabel,
		analyzeBtn,
	)

	return container.NewBorder(header, footer, nil, nil, periodsScroll)
}
