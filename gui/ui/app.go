package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"gopro-gui/config"
	"gopro-gui/ffmpeg"
	"gopro-gui/metadata"
)

// App represents the main application
type App struct {
	fyneApp fyne.App
	window  fyne.Window
	ff      *ffmpeg.FFmpeg
	cfg     *config.Config

	// Shared state between steps
	extractedMetadataFiles []string          // Metadata files created in Step 1
	periods                []metadata.Period // Periods configured in Step 2
	analysisResult         *metadata.AnalysisResult
	extractedClips         []string // Clip files created in Step 3

	// Tab references for status updates
	tabs     *container.AppTabs
	tabItems []*container.TabItem
}

// NewApp creates a new application instance
func NewApp() (*App, error) {
	ff, err := ffmpeg.New()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	return &App{
		ff:  ff,
		cfg: cfg,
	}, nil
}

// Run starts the application
func (a *App) Run() {
	a.fyneApp = app.NewWithID("com.gopro-clip-extractor")
	a.window = a.fyneApp.NewWindow("GoPro Clip Extractor")
	a.window.Resize(fyne.NewSize(1000, 700))

	// Create tab items and store references for status updates
	a.tabItems = []*container.TabItem{
		container.NewTabItem("1. Extract Metadata", a.createStep1()),
		container.NewTabItem("2. Configure Periods", a.createStep2()),
		container.NewTabItem("3. Extract Clips", a.createStep3()),
		container.NewTabItem("4. Edit Clips", a.createStep4Edit()),
		container.NewTabItem("5. Combine", a.createStep5()),
	}

	// Create the tabbed interface
	a.tabs = container.NewAppTabs(a.tabItems...)
	a.tabs.SetTabLocation(container.TabLocationTop)

	a.window.SetContent(a.tabs)
	a.window.SetOnClosed(func() {
		a.cfg.Save()
	})

	a.window.ShowAndRun()
}

// markStepComplete updates a tab title to show completion status
func (a *App) markStepComplete(stepIndex int) {
	if stepIndex < 0 || stepIndex >= len(a.tabItems) {
		return
	}
	titles := []string{
		"1. Extract Metadata",
		"2. Configure Periods",
		"3. Extract Clips",
		"4. Edit Clips",
		"5. Combine",
	}
	a.tabItems[stepIndex].Text = titles[stepIndex] + " ✓"
	a.tabs.Refresh()
}

// markStepIncomplete resets a tab title to show incomplete status
func (a *App) markStepIncomplete(stepIndex int) {
	if stepIndex < 0 || stepIndex >= len(a.tabItems) {
		return
	}
	titles := []string{
		"1. Extract Metadata",
		"2. Configure Periods",
		"3. Extract Clips",
		"4. Edit Clips",
		"5. Combine",
	}
	a.tabItems[stepIndex].Text = titles[stepIndex]
	a.tabs.Refresh()
}

// showError displays an error dialog
func (a *App) showError(title, message string) {
	dialog := widget.NewLabel(message)
	popup := a.fyneApp.NewWindow(title)
	popup.SetContent(container.NewVBox(
		dialog,
		widget.NewButton("OK", func() {
			popup.Close()
		}),
	))
	popup.Resize(fyne.NewSize(400, 150))
	popup.Show()
}

// showInfo displays an info dialog
func (a *App) showInfo(title, message string) {
	dialog := widget.NewLabel(message)
	popup := a.fyneApp.NewWindow(title)
	popup.SetContent(container.NewVBox(
		dialog,
		widget.NewButton("OK", func() {
			popup.Close()
		}),
	))
	popup.Resize(fyne.NewSize(400, 150))
	popup.Show()
}
