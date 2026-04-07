// ================ Version : V1.1.4 ===========
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

// RotationMode controls how log files are created and rotated.
type RotationMode string

const (
	// RotationDaily: one log file per calendar day (e.g. app_02-01-2006.log).
	// If the container restarts within the same day, logs are appended to the
	// existing file. A new file is created automatically at midnight.
	RotationDaily RotationMode = "daily"

	// RotationPerRun: a new log file is created on every program start
	// (e.g. app_02-01-2006_150405.log). Restarts never share a file.
	RotationPerRun RotationMode = "perrun"
)

// CofigLogger is the configuration passed to InitLogger.
//
// Use it to control the log level, console formatting, file output, rotation
// mode, and log retention settings.
type CofigLogger struct {
	LogLevel    string
	LogToFile   bool
	LogFileName string
	Formatted   bool // true = JSON everywhere, false = pretty everywhere

	// ── Rotation ─────────────────────────────────────────────────────────────
	// RotationMode selects the file-naming / rotation strategy:
	//   RotationDaily   – one file per day; survives container restarts.
	//   RotationPerRun  – new file on every startup (default when empty).
	RotationMode RotationMode

	// MaxFileSize is the maximum size (MB) of a single log file before
	// lumberjack rolls it over with a numeric suffix.
	// 0 → lumberjack default (100 MB).
	MaxFileSize int

	// MaxLogFiles is the maximum number of .log files kept in the log
	// directory. Oldest files are removed when the limit is exceeded.
	// 0 → no limit enforced by astrolog (lumberjack still manages its own
	// compressed backups independently).
	MaxLogFiles int

	// MaxAgeDays is the maximum age (in days) of log files to retain.
	// Files older than this are deleted on each InitLogger call.
	// 0 → no age-based deletion.
	MaxAgeDays int
}

// =============================
// Console Writer
// =============================

// ConsoleWriterWithLevel wraps zerolog.ConsoleWriter.
type ConsoleWriterWithLevel struct {
	zerolog.ConsoleWriter
}

func (c ConsoleWriterWithLevel) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	_, err := c.ConsoleWriter.Write(p)
	return len(p), err
}

// =============================
// File Writer
// =============================

type FileWriterWithLevel struct {
	*lumberjack.Logger
	Formatted bool
}

func (f FileWriterWithLevel) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	// Formatted == true  → RAW JSON
	if f.Formatted {
		return f.Logger.Write(p)
	}
	// Formatted == false → pretty formatted
	formatted, err := formatLogEntry(level, p)
	if err != nil {
		return f.Logger.Write(p)
	}
	_, err = f.Logger.Write([]byte(formatted))
	return len(p), err
}

// =============================
// Formatting Helpers
// =============================

func formatLogEntry(level zerolog.Level, p []byte) (string, error) {
	var entry map[string]interface{}
	if err := json.Unmarshal(p, &entry); err != nil {
		return "", err
	}

	timestamp, _ := entry["time"].(string)
	message, _ := entry["message"].(string)
	caller, _ := entry["caller"].(string)

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

func stripCallerPath(file string) string {
	if file == "" {
		return file
	}
	parts := strings.Split(file, "/")
	base := parts[len(parts)-1]
	base = strings.TrimSuffix(base, ".go")
	return base
}

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
	sort.Strings(extras)
	return extras
}

// =============================
// Run Separator
// =============================

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

	banner := fmt.Sprintf("\n%s\n%s\n%s\n%s\n%s\n\n", top, mid, div, row1, bottom)
	_, _ = lj.Write([]byte(banner))
}

// =============================
// File Cleanup — count-based
// =============================

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

	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := logFiles[i].Info()
		infoJ, _ := logFiles[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	for _, file := range logFiles[:len(logFiles)-maxFiles] {
		_ = os.Remove(filepath.Join(logDir, file.Name()))
	}
	return nil
}

// =============================
// File Cleanup — age-based
// =============================

func deleteAgedLogFiles(logDir string, maxAgeDays int) error {
	if maxAgeDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(logDir, entry.Name()))
		}
	}
	return nil
}

// =============================
// File-name resolution
// =============================

// resolveLogFilename returns the log file path for the current run.
//
//   - RotationDaily  → <base>_DD-MM-YYYY.log
//     The file is opened in append mode; if it already exists (e.g. after a
//     container restart), new log lines are appended to it.
//
//   - RotationPerRun → <base>_DD-MM-YYYY_HHMMSS.log
//     A unique name is generated at startup so every run gets its own file.
func resolveLogFilename(cfg CofigLogger, logDir string) (string, bool) {
	now := time.Now()

	switch cfg.RotationMode {
	case RotationDaily:
		name := fmt.Sprintf("%s_%s.log",
			cfg.LogFileName,
			now.Format("02-01-2006"),
		)
		// Signal to the caller that the file may already exist.
		fullPath := filepath.Join(logDir, name)
		_, err := os.Stat(fullPath)
		fileExists := err == nil
		return fullPath, fileExists

	default: // RotationPerRun (and empty string)
		name := fmt.Sprintf("%s_%s_%s.log",
			cfg.LogFileName,
			now.Format("02-01-2006"),
			now.Format("150405"),
		)
		return filepath.Join(logDir, name), false
	}
}

// ==============
//
//	Init Logger
//
// ==============
//
// InitLogger configures the global zerolog logger based on the provided CofigLogger.
// Missing or zero-value fields are filled in by applyDefaults before any setup begins.
//
// Default Values :
//
//		CofigLogger{
//				LogLevel:     "info",
//				LogToFile:    false,
//				LogFileName:  "app",
//				Formatted:    false,
//				RotationMode: RotationPerRun,
//				MaxFileSize:  5, // MB
//				MaxLogFiles:  30,
//				MaxAgeDays:   30,
//	}
//
// What it does, in order:
//
//  1. Applies default values to any unset config fields.
//  2. Sets the global time format and uses local time for all timestamps.
//  3. Strips the full file path from caller info, keeping only "file:line".
//  4. Builds the list of output writers (console and/or file).
//  5. Replaces the global log.Logger with the newly built logger.
//  6. Sets the global log level (e.g. debug, info, warn, error).
//
// Call this once at application startup, before any logging takes place.
//
// Example:
//
//	astrolog.InitLogger(astrolog.CofigLogger{
//	    LogLevel:  "debug",
//	    LogToFile: true,
//	})
func InitLogger(cfg CofigLogger) {
	// Fill in any zero/empty fields with sensible defaults.
	cfg = applyDefaults(cfg)

	// Use a fixed timestamp layout across all log entries.
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"

	// Always stamp log entries with the local wall clock, not UTC.
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
	}

	// Shorten caller info from a full path to "filename:line".
	// e.g. "/home/user/project/pkg/server.go:42" → "server:42"
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return fmt.Sprintf("%s:%d", stripCallerPath(file), line)
	}

	var writers []io.Writer

	// ── Console ──────────────────────────────────────────────────────────────
	// Formatted == true  → write raw JSON directly to stderr.
	// Formatted == false → write human-readable, colorized output to stderr.
	if cfg.Formatted {
		writers = append(writers, os.Stderr)
	} else {
		writers = append(writers, buildConsoleWriter())
	}

	// ── File ─────────────────────────────────────────────────────────────────
	// Only set up file logging when explicitly requested.
	// buildFileWriter handles directory creation, rotation, and cleanup;
	// it returns nil if the file cannot be opened, in which case we skip it
	// silently and continue with console-only output.
	if cfg.LogToFile {
		if fw := buildFileWriter(cfg); fw != nil {
			writers = append(writers, fw)
		}
	}

	// Lock before touching the global logger so concurrent goroutines
	// that call GetLogger() or UpdateLogLevel() at the same time are safe.
	mu.Lock()
	defer mu.Unlock()

	// Wire up the global logger with all active writers, a timestamp field,
	// and a caller field on every log entry.
	log.Logger = zerolog.New(zerolog.MultiLevelWriter(writers...)).
		With().
		Timestamp().
		Caller().
		Logger()

	// Apply the log level last; this is a global zerolog setting that
	// filters out entries below the chosen severity across all writers.
	UpdateLogLevel(cfg.LogLevel)
}

// =============================
// Console Builder
// =============================

// buildConsoleWriter creates the pretty console writer used when Formatted is false.
func buildConsoleWriter() ConsoleWriterWithLevel {
	return ConsoleWriterWithLevel{
		ConsoleWriter: zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05.000",
			FormatCaller: func(i interface{}) string {
				caller, _ := i.(string)
				return "\033[34m" + caller + "\033[0m"
			},
		},
	}
}

// ---------------------------
// DEFAULT CONFIG
////////////////////////////

// DefaultConfig returns the default logger configuration used by InitLogger.
func DefaultConfig() CofigLogger {
	return CofigLogger{
		LogLevel:     "info",
		LogToFile:    false,
		LogFileName:  "app",
		Formatted:    false,
		RotationMode: RotationPerRun,
		MaxFileSize:  5, // MB
		MaxLogFiles:  30,
		MaxAgeDays:   30,
	}
}

// applyDefaults fills any zero-value fields in cfg with DefaultConfig values.
func applyDefaults(cfg CofigLogger) CofigLogger {
	def := DefaultConfig()

	if cfg.LogLevel == "" {
		cfg.LogLevel = def.LogLevel
	}
	if cfg.LogFileName == "" {
		cfg.LogFileName = def.LogFileName
	}
	if cfg.RotationMode == "" {
		cfg.RotationMode = def.RotationMode
	}
	if cfg.MaxFileSize == 0 {
		cfg.MaxFileSize = def.MaxFileSize
	}
	if cfg.MaxLogFiles == 0 {
		cfg.MaxLogFiles = def.MaxLogFiles
	}
	if cfg.MaxAgeDays == 0 {
		cfg.MaxAgeDays = def.MaxAgeDays
	}
	return cfg
}

// =============================
// File Builder
// =============================

// buildFileWriter creates the file writer and prepares log rotation and cleanup.
func buildFileWriter(cfg CofigLogger) *FileWriterWithLevel {
	logDir := "./logs"
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil
	}

	// Run cleanup before opening/creating any file.
	_ = deleteAgedLogFiles(logDir, cfg.MaxAgeDays)
	_ = deleteOldLogFiles(logDir, cfg.MaxLogFiles)

	fullPath, fileExists := resolveLogFilename(cfg, logDir)

	lj := &lumberjack.Logger{
		Filename:   fullPath,
		MaxSize:    cfg.MaxFileSize, // MB; 0 → lumberjack default (100 MB)
		MaxBackups: 3,
		MaxAge:     cfg.MaxAgeDays,
	}

	// Write the run-separator banner.
	// For daily mode, append a restart marker when the file already exists.
	if fileExists {
		writeRestartSeparator(lj)
	} else {
		writeRunSeparator(lj)
	}

	return &FileWriterWithLevel{
		Logger:    lj,
		Formatted: cfg.Formatted,
	}
}

// writeRestartSeparator is written into an existing daily log file when the
// process restarts within the same day.
func writeRestartSeparator(lj *lumberjack.Logger) {
	now := time.Now()
	line1 := fmt.Sprintf("  Restarted : %s", now.Format("2006-01-02 15:04:05"))
	width := 50
	if len(line1)+4 > width {
		width = len(line1) + 4
	}
	top := "┌" + strings.Repeat("─", width) + "┐"
	mid := "│" + fmt.Sprintf("%-*s", width, "  ↺  PROCESS RESTARTED") + "│"
	div := "├" + strings.Repeat("─", width) + "┤"
	row1 := "│" + fmt.Sprintf("%-*s", width, line1) + "│"
	bottom := "└" + strings.Repeat("─", width) + "┘"

	banner := fmt.Sprintf("\n%s\n%s\n%s\n%s\n%s\n\n", top, mid, div, row1, bottom)
	_, _ = lj.Write([]byte(banner))
}

// =============================
// Log Level
// =============================

// GetLogger returns the current global zerolog logger.
func GetLogger() zerolog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return log.Logger
}

// UpdateLogLevel changes the global zerolog level at runtime.
func UpdateLogLevel(level string) {
	parsed, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsed = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsed)
}
