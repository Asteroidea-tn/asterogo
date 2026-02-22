package astrolog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

var mu sync.Mutex

type CofigLogger struct {
	LogLevel    string
	LogToFile   bool
	LogFileName string
	Formatted   bool
	MaxFileSize int
	MaxLogFiles int
}

// =============================
// Console Writer
// =============================

// ConsoleWriterWithLevel wraps zerolog.ConsoleWriter to satisfy the LevelWriter interface.
type ConsoleWriterWithLevel struct {
	zerolog.ConsoleWriter
}

// WriteLevel implements zerolog.LevelWriter for the console.
// We must return len(p) — not the bytes written by ConsoleWriter — because
// ConsoleWriter transforms JSON into a human-readable string of a different length.
// Returning the transformed length causes zerolog to panic with "short write".
func (c ConsoleWriterWithLevel) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	_, err := c.ConsoleWriter.Write(p)
	return len(p), err
}

// =============================
// File Writer
// =============================

// FileWriterWithLevel wraps lumberjack.Logger to satisfy the LevelWriter interface.
// When Formatted is enabled, it parses the JSON log entry and writes a human-readable line instead.
type FileWriterWithLevel struct {
	*lumberjack.Logger
	Formatted bool
}

// WriteLevel implements zerolog.LevelWriter for the file.
// Always return len(p) back to zerolog to avoid "short write" errors.
func (f FileWriterWithLevel) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if !f.Formatted {
		return f.Logger.Write(p)
	}
	formatted, err := formatLogEntry(level, p)
	if err != nil {
		// Fallback to raw JSON if parsing fails
		return f.Logger.Write(p)
	}
	// Write the formatted bytes and report len(p) back to zerolog.
	// lumberjack.Write returns the number of formatted bytes written, not len(p),
	// which would cause a "short write". We report len(p) to satisfy zerolog.
	_, err = f.Logger.Write([]byte(formatted))
	return len(p), err
}

// =============================
// Formatting Helpers
// =============================

// formatLogEntry parses a zerolog JSON entry and returns a human-readable log line.
func formatLogEntry(level zerolog.Level, p []byte) (string, error) {
	var entry map[string]interface{}
	if err := json.Unmarshal(p, &entry); err != nil {
		return "", err
	}

	timestamp, _ := entry["time"].(string)
	message, _ := entry["message"].(string)
	caller, _ := entry["caller"].(string)

	// Truncate timestamp to millisecond precision (e.g. "2006-01-02 15:04:05.000")
	formattedTimestamp := timestamp
	if len(timestamp) >= 22 {
		formattedTimestamp = strings.ReplaceAll(timestamp, "T", " ")[:22]
	}

	extras := collectExtraFields(entry)

	return fmt.Sprintf("%s | %-5s | %-25s | %s | %s\n",
		formattedTimestamp,
		level.String(),
		caller,
		message,
		strings.Join(extras, " "),
	), nil
}

// stripCallerPath takes a full caller string (e.g. "pkg/sub/file.go:42")
// and returns just the filename without the .go extension (e.g. "file:42").
func stripCallerPath(file string) string {
	if file == "" {
		return file
	}
	parts := strings.Split(file, "/")
	base := parts[len(parts)-1]
	base = strings.TrimSuffix(base, ".go")
	return base
}

// collectExtraFields returns key=value pairs for any fields beyond the standard zerolog ones.
func collectExtraFields(entry map[string]interface{}) []string {
	standard := map[string]bool{
		"time":    true,
		"message": true,
		"level":   true,
		"caller":  true,
	}
	var extras []string
	for k, v := range entry {
		if !standard[k] {
			extras = append(extras, fmt.Sprintf("%s=%v", k, v))
		}
	}
	sort.Strings(extras) // deterministic output order
	return extras
}

// =============================
// Run Separator
// =============================

// writeRunSeparator writes a decorative box banner to the log file marking a new process start.
func writeRunSeparator(lj *lumberjack.Logger) {
	now := time.Now()

	line1 := fmt.Sprintf("  Started : %s", now.Format("2006-01-02 15:04:05"))

	width := 50
	if len(line1)+4 > width {
		width = len(line1) + 4
	}

	top := "┌" + strings.Repeat("─", width) + "┐"
	mid := "│" + fmt.Sprintf("%-*s", width, "  ▶  PROGRAM STARTED") + "│"
	div := "├" + strings.Repeat("─", width) + "┤"
	row1 := "│" + fmt.Sprintf("%-*s", width, line1) + "│"
	bottom := "└" + strings.Repeat("─", width) + "┘"

	banner := fmt.Sprintf("\n%s\n%s\n%s\n%s\n%s\n\n",
		top, mid, div, row1, bottom,
	)

	_, _ = lj.Write([]byte(banner))
}

// =============================
// File Cleanup
// =============================

// deleteOldLogFiles removes the oldest log files in logDir when the count exceeds maxFiles.
func deleteOldLogFiles(logDir string, maxFiles int) error {
	if maxFiles <= 0 {
		return nil
	}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	var logFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			logFiles = append(logFiles, entry)
		}
	}

	if len(logFiles) <= maxFiles {
		return nil
	}

	// Sort oldest first so we can trim from the front
	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := logFiles[i].Info()
		infoJ, _ := logFiles[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	toDelete := logFiles[:len(logFiles)-maxFiles]
	for _, file := range toDelete {
		if err := os.Remove(filepath.Join(logDir, file.Name())); err != nil {
			log.Err(err).Msgf("Failed to delete old log file: %s", file.Name())
		}
	}
	return nil
}

// =============================
// Init Logger
// =============================

// InitLogger sets up the global zerolog logger with console and optional file output.
func InitLogger(cfg CofigLogger) {
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
	}

	// Override the caller marshaler globally so the caller field is clean
	// (no full path, no .go extension) for ALL writers before it hits any of them.
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return fmt.Sprintf("%s:%d", stripCallerPath(file), line)
	}

	writers := []io.Writer{buildConsoleWriter()}

	if cfg.LogToFile {
		if fw := buildFileWriter(cfg); fw != nil {
			writers = append(writers, fw)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	log.Logger = zerolog.New(zerolog.MultiLevelWriter(writers...)).
		With().
		Timestamp().
		Caller().
		Logger()

	UpdateLogLevel(cfg.LogLevel)
}

// =============================
// Console Builder
// =============================

// buildConsoleWriter creates the styled console writer.
func buildConsoleWriter() ConsoleWriterWithLevel {
	return ConsoleWriterWithLevel{
		ConsoleWriter: zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05.000",
			// CallerMarshalFunc already cleaned the value — just add color.
			FormatCaller: func(i interface{}) string {
				caller, _ := i.(string)
				return "\033[34m" + caller + "\033[0m"
			},
		},
	}
}

// =============================
// File Builder
// =============================

// buildFileWriter creates the rotating file writer, returning nil on setup failure.
// Files are named by date only (e.g. "app_02-01-2006.log") so that multiple
// runs on the same day append to the same file instead of creating a new one.
func buildFileWriter(cfg CofigLogger) *FileWriterWithLevel {
	logDir := "./logs"
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Err(err).Msgf("Failed to create log directory: %v", err)
		return nil
	}

	if err := deleteOldLogFiles(logDir, cfg.MaxLogFiles); err != nil {
		log.Err(err).Msgf("Failed to clean old log files: %v", err)
	}

	suffix := "_json"
	if cfg.Formatted {
		suffix = ""
	}

	// Date only — same file is reused for every run within the same day
	filename := fmt.Sprintf("%s_%s%s.log",
		cfg.LogFileName,
		time.Now().Format("02-01-2006"),
		suffix,
	)

	lj := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, filename),
		MaxSize:    cfg.MaxFileSize,
		MaxBackups: 3,
		MaxAge:     30,
	}

	// Write the decorative run separator so each restart is clearly visible in the file
	writeRunSeparator(lj)

	return &FileWriterWithLevel{
		Logger:    lj,
		Formatted: cfg.Formatted,
	}
}

// =============================
// Log Level
// =============================

// GetLogger returns the current global logger instance.
func GetLogger() zerolog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return log.Logger
}

// UpdateLogLevel sets the global zerolog level from a string (e.g. "debug", "warn").
// Defaults to info if the string is unrecognized.
func UpdateLogLevel(level string) {
	parsed, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsed = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsed)
}
