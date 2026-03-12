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

// =============================
// Init Logger
// =============================

func InitLogger(cfg CofigLogger) {
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
	}
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return fmt.Sprintf("%s:%d", stripCallerPath(file), line)
	}

	var writers []io.Writer

	// ── Console ──────────────────────────────────────────────────────────────
	if cfg.Formatted {
		writers = append(writers, os.Stderr) // raw JSON
	} else {
		writers = append(writers, buildConsoleWriter()) // pretty
	}

	// ── File ─────────────────────────────────────────────────────────────────
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

// =============================
// File Builder
// =============================

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

func GetLogger() zerolog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return log.Logger
}

func UpdateLogLevel(level string) {
	parsed, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsed = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(parsed)
}
