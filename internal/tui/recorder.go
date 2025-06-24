package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Recorder captures TUI state changes and renders for debugging.
type Recorder struct {
	logFile  *os.File
	frameDir string
	frameNum int
	enabled  bool
}

// NewRecorder creates a TUI state recorder.
func NewRecorder(enabled bool) *Recorder {
	if !enabled {
		return &Recorder{enabled: false}
	}

	// Create recording directory
	recordDir := filepath.Join(os.TempDir(), fmt.Sprintf("tui-record-%d", time.Now().Unix()))
	if err := os.MkdirAll(recordDir, 0750); err != nil {
		return &Recorder{enabled: false}
	}

	// Create log file
	logPath := filepath.Join(recordDir, "tui.log")
	logFile, err := os.Create(filepath.Clean(logPath)) // #nosec G304 -- safe constructed path
	if err != nil {
		return &Recorder{enabled: false}
	}

	r := &Recorder{
		enabled:  true,
		logFile:  logFile,
		frameDir: recordDir,
		frameNum: 0,
	}

	r.Log("TUI Recorder started at %s", recordDir)
	return r
}

// RecordState captures the current state.
func (r *Recorder) RecordState(model Model, msg tea.Msg) {
	if !r.enabled {
		return
	}

	r.frameNum++

	// Log the message
	r.Log("\n=== Frame %d ===", r.frameNum)
	r.Log("Time: %s", time.Now().Format("15:04:05.000"))
	r.Log("Message Type: %T", msg)
	r.Log("Message: %#v", msg)
	r.Log("State: %v", model.state)
	r.Log("Ready: %v", model.ready)
	r.Log("Pending: %d items", len(model.pending))

	// Save the rendered view
	view := model.View()
	framePath := filepath.Join(r.frameDir, fmt.Sprintf("frame-%04d.txt", r.frameNum))
	if err := os.WriteFile(framePath, []byte(view), 0600); err != nil {
		r.Log("Error saving frame: %v", err)
	}

	// Also log the view
	r.Log("\n--- Rendered View ---\n%s\n--- End View ---", view)
}

// Log writes to the log file.
func (r *Recorder) Log(format string, args ...any) {
	if !r.enabled || r.logFile == nil {
		return
	}

	if _, err := fmt.Fprintf(r.logFile, format+"\n", args...); err != nil {
		return
	}
	if err := r.logFile.Sync(); err != nil {
		return
	}
}

// Close closes the recorder.
func (r *Recorder) Close() {
	if r.logFile != nil {
		r.Log("Recording complete. %d frames captured.", r.frameNum)
		r.Log("View recording at: %s", r.frameDir)
		_ = r.logFile.Close() // Best effort close
	}
}
