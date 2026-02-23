// ================ Version : V1.1.2 ===========
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
	Formatted   bool // true = JSON everywhere, false = pretty everywhere
	MaxFileSize int
	MaxLogFiles int
}

//
// =============================
// Console Writer
// =============================
//

// ConsoleWriterWithLevel wraps zerolog.ConsoleWriter
type ConsoleWriterWithLevel struct {
	zerolog.ConsoleWriter
}

func (c ConsoleWriterWithLevel) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	_, err := c.ConsoleWriter.Write(p)
	return len(p), err
}

//
// =============================
// File Writer
// =============================
//

type FileWriterWithLevel struct {
	*lumberjack.Logger
	Formatted bool
}

func (f FileWriterWithLevel) WriteLevel(level zerolog.Level, p []byte) (int, error) {

	// Formatted == true  → RAW JSON
	if f.Formatted {
		return f.Logger.Write(p)
	}

	// Formatted == false → Pretty formatted
	formatted, err := formatLogEntry(level, p)
	if err != nil {
		return f.Logger.Write(p)
	}

	_, err = f.Logger.Write([]byte(formatted))
	return len(p), err
}

//
// =============================
// Formatting Helpers
// =============================
//

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

//
// =============================
// Run Separator
// =============================
//

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

//
// =============================
// File Cleanup
// =============================
//

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

	toDelete := logFiles[:len(logFiles)-maxFiles]
	for _, file := range toDelete {
		_ = os.Remove(filepath.Join(logDir, file.Name()))
	}

	return nil
}

//
// =============================
// Init Logger
// =============================
//

func InitLogger(cfg CofigLogger) {

	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
	}

	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return fmt.Sprintf("%s:%d", stripCallerPath(file), line)
	}

	var writers []io.Writer

	// =============================
	// Console
	// =============================

	if cfg.Formatted {
		// JSON to console
		writers = append(writers, os.Stderr)
	} else {
		// Pretty console
		writers = append(writers, buildConsoleWriter())
	}

	// =============================
	// File
	// =============================

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

//
// =============================
// Console Builder
// =============================
//

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

//
// =============================
// File Builder
// =============================
//

func buildFileWriter(cfg CofigLogger) *FileWriterWithLevel {

	logDir := "./logs"
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil
	}

	_ = deleteOldLogFiles(logDir, cfg.MaxLogFiles)

	filename := fmt.Sprintf("%s_%s.log",
		cfg.LogFileName,
		time.Now().Format("02-01-2006"),
	)

	lj := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, filename),
		MaxSize:    cfg.MaxFileSize,
		MaxBackups: 3,
		MaxAge:     30,
	}

	writeRunSeparator(lj)

	return &FileWriterWithLevel{
		Logger:    lj,
		Formatted: cfg.Formatted,
	}
}

//
// =============================
// Log Level
// =============================
//

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
