package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"os"
)

const (
	logMaxBytes = 1024 * 1024
	logLineMax  = 600
)

// Logger writes single-line sanitized diagnostics to <userData>/logs/sisy.log.
// Task titles/bodies must never be passed to Logger.
type Logger struct {
	mu   sync.Mutex
	path string
}

func NewLogger(logDir string) *Logger {
	return &Logger{path: filepath.Join(logDir, "sisy.log")}
}

func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Logger) Info(parts ...any) { l.write("INFO", parts...) }
func (l *Logger) Warn(parts ...any) { l.write("WARN", parts...) }
func (l *Logger) Error(parts ...any) { l.write("ERROR", parts...) }

func (l *Logger) write(level string, parts ...any) {
	if l == nil || l.path == "" {
		return
	}
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(fmt.Sprint(p))
	}
	line := strings.ReplaceAll(sb.String(), "\r\n", " ⏎ ")
	line = strings.ReplaceAll(line, "\n", " ⏎ ")
	runes := []rune(line)
	if len(runes) > logLineMax {
		line = string(runes[:logLineMax])
	}
	entry := fmt.Sprintf("%s [%s] %s\n", time.Now().Format(time.RFC3339), level, line)

	l.mu.Lock()
	defer l.mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(l.path), 0o755)
	if st, err := os.Stat(l.path); err == nil && st.Size() > logMaxBytes {
		_ = os.Rename(l.path, l.path+".1")
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(entry)
	_ = f.Close()
}
