package logger

import (
	"fmt"
	"io"
	stdlog "log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"avyos.dev/lib/format"
)

// Level represents the log severity level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with levels and components
type Logger struct {
	component string
	level     Level
	writer    io.Writer
	errWriter io.Writer
	mutex     sync.Mutex
	fields    map[string]any
}

var (
	globalWriter    io.Writer = os.Stdout
	globalErrWriter io.Writer = os.Stderr
	globalWriterMu  sync.RWMutex
	setupOnce       sync.Once
	setupErr        error
)

// Default is the default logger instance
var Default = New("")

// New creates a new logger for the given component
func New(component string) *Logger {
	return &Logger{
		component: component,
		level:     getDefaultLevel(),
		writer:    globalWriter,
		errWriter: globalErrWriter,
		fields:    make(map[string]any),
	}
}

// SetupLog configures process logging to append to a per-process log file
// while preserving stdout/stderr output.
//
// Root/system process path:
//
//	/var/cache/log/libexec/<name>.log
//
// User process path:
//
//	$HOME/.cache/log/libexec/<name>.log
func SetupLog() error {
	setupOnce.Do(func() {
		logPath, err := resolveServiceLogPath()
		if err != nil {
			setupErr = err
			return
		}
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			setupErr = err
			return
		}
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			setupErr = err
			return
		}

		globalWriterMu.Lock()
		globalWriter = io.MultiWriter(os.Stdout, logFile)
		globalErrWriter = io.MultiWriter(os.Stderr, logFile)
		globalWriterMu.Unlock()
		Default.writer = globalWriter
		Default.errWriter = globalErrWriter
		stdlog.SetOutput(globalErrWriter)
		stdlog.SetFlags(stdlog.LstdFlags | stdlog.Lmicroseconds)
	})
	return setupErr
}

func resolveServiceLogPath() (string, error) {
	name := sanitizeServiceName(guessProcessName())
	if name == "" {
		name = "unknown"
	}
	fileName := name + ".log"

	if os.Geteuid() == 0 {
		return filepath.Join("/var/cache/log/libexec", fileName), nil
	}

	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(home, ".cache", "log", "services", fileName), nil
}

func guessProcessName() string {
	exe := filepath.Clean(strings.TrimSpace(os.Args[0]))
	if exe == "" {
		return "unknown"
	}

	base := filepath.Base(exe)
	if base == "exec" {
		parent := filepath.Base(filepath.Dir(exe))
		if parent != "" && parent != "." && parent != string(os.PathSeparator) {
			return parent
		}
	}
	return base
}

func sanitizeServiceName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown"
	}
	return out
}

// getDefaultLevel returns the default log level based on LOG_LEVEL env var
func getDefaultLevel() Level {
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.level = level
}

// With returns a new logger with the given field attached
func (l *Logger) With(key string, value any) *Logger {
	newFields := make(map[string]any, len(l.fields)+1)
	maps.Copy(newFields, l.fields)
	newFields[key] = value

	return &Logger{
		component: l.component,
		level:     l.level,
		writer:    l.writer,
		errWriter: l.errWriter,
		fields:    newFields,
	}
}

// log writes a log message at the given level
func (l *Logger) log(level Level, format string, args ...any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if level < l.level {
		return
	}

	// Build timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Build level string with color
	levelStr := l.formatLevel(level)

	// Build component tag
	componentTag := ""
	if l.component != "" {
		componentTag = fmt.Sprintf("[%s]", l.component)
	}

	// Build message
	message := fmt.Sprintf(format, args...)

	// Build fields string
	fieldsStr := ""
	if len(l.fields) > 0 {
		var parts []string
		for k, v := range l.fields {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		fieldsStr = " " + strings.Join(parts, " ")
	}

	// Format final output
	output := fmt.Sprintf("[%s] %s %s %s%s\n", timestamp, levelStr, componentTag, message, fieldsStr)

	// Write to appropriate output
	globalWriterMu.RLock()
	writer := globalWriter
	if level == ERROR || level == WARN {
		writer = globalErrWriter
	}
	globalWriterMu.RUnlock()
	if writer == nil {
		writer = l.writer
	}

	writer.Write([]byte(output))
}

// formatLevel formats the log level with appropriate colors
func (l *Logger) formatLevel(level Level) string {
	levelStr := fmt.Sprintf("[%s]", level.String())

	// Apply colors using format package
	switch level {
	case DEBUG:
		return format.Color(format.Dim, levelStr)
	case INFO:
		return format.Color(format.Cyan, levelStr)
	case WARN:
		return format.Color(format.Yellow, levelStr)
	case ERROR:
		return format.Color(format.Red, levelStr)
	default:
		return levelStr
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...any) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...any) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...any) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...any) {
	l.log(ERROR, format, args...)
}

// Global convenience functions using the default logger

// Debug logs a debug message using the default logger
func Debug(format string, args ...any) {
	Default.Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...any) {
	Default.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...any) {
	Default.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...any) {
	Default.Error(format, args...)
}
