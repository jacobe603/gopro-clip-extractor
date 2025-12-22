package main

import (
	"fmt"
	"os"

	"gopro-gui/ui"
)

func main() {
	app, err := ui.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing app: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nPlease ensure ffmpeg.exe and ffprobe.exe are in the bin/ folder.\n")
		fmt.Fprintf(os.Stderr, "Download from: https://www.gyan.dev/ffmpeg/builds/\n")
		os.Exit(1)
	}

	app.Run()
}
