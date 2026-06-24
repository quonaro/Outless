package singbox

import (
	singlog "github.com/sagernet/sing-box/log"
)

// singBoxLogWriter implements sing-box log.PlatformWriter and forwards
// formatted messages to an output callback.
type singBoxLogWriter struct {
	output func(string)
}

// newSingBoxLogWriter creates a writer that forwards sing-box log lines.
func newSingBoxLogWriter(output func(string)) *singBoxLogWriter {
	return &singBoxLogWriter{output: output}
}

func (w *singBoxLogWriter) DisableColors() bool { return true }

func (w *singBoxLogWriter) WriteMessage(level singlog.Level, message string) {
	if w.output == nil {
		return
	}
	w.output("[" + singlog.FormatLevel(level) + "] " + message)
}
