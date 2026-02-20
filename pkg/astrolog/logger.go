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

// ConsoleWriterWithLevel wraps zerolog.ConsoleWriter to satisfy the LevelWriter interface.
type ConsoleWriterWithLevel struct {
	zerolog.ConsoleWriter
}

func (c ConsoleWriterWithLevel) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	return c.ConsoleWriter.Write(p)
}

// FileWriterWithLevel wraps lumberjack.Logger to satisfy the LevelWriter interface.
// When Formatted is enabled, it parses the JSON log entry and writes a human-readable line instead.
type FileWriterWithLevel struct {
	*lumberjack.Logger
	Formatted bool
}

// WriteLevel implements zerolog.LevelWriter with the correct 2-argument signature.
// zerolog calls this method instead of Write when the writer implements LevelWriter.
func (f FileWriterWithLevel) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if !f.Formatted {
		return f.Logger.Write(p)
	}
	formatted, err := formatLogEntry(level, p)
	if err != nil {
		// Fallback to raw JSON if parsing fails
		return f.Logger.Write(p)
	}
	return f.Logger.Write([]byte(formatted))
}

// formatLogEntry parses a zerolog JSON entry and returns a human-readable log line.
func formatLogEntry(level zerolog.Level, p []byte) (string, error) {
	var entry map[string]interface{}
	if err := json.Unmarshal(p, &entry); err != nil {
		return "", err
	}

	timestamp, _ := entry["time"].(string)
	message, _ := entry["message"].(string)
	caller, _ := entry["caller"].(string)

	// Extract just the filename without path or extension
	callerFile := stripCallerPath(caller)

	// Truncate timestamp to millisecond precision (e.g. "2006-01-02 15:04:05.000")
	formattedTimestamp := timestamp
	if len(timestamp) >= 22 {
		formattedTimestamp = strings.ReplaceAll(timestamp, "T", " ")[:22]
	}

	// Collect any extra fields beyond the standard ones
	extras := collectExtraFields(entry)

	return fmt.Sprintf("%s | %s | %s | %s | %s\n",
		formattedTimestamp,
		level.String(),
		callerFile,
		message,
		strings.Join(extras, " "),
	), nil
}

// stripCallerPath takes a full caller string (e.g. "pkg/sub/file.go:42")
// and returns just the filename without the .go extension (e.g. "file:42").
func stripCallerPath(caller string) string {
	if caller == "" {
		return caller
	}
	parts := strings.Split(caller, "/")
	file := parts[len(parts)-1]
	file = strings.TrimSuffix(file, ".go")
	return file
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

// InitLogger sets up the global zerolog logger with console and optional file output.
func InitLogger(cfg CofigLogger) {
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
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

// buildConsoleWriter creates the styled console writer.
func buildConsoleWriter() ConsoleWriterWithLevel {
	return ConsoleWriterWithLevel{
		ConsoleWriter: zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "2006-01-02 15:04:05.000",
			FormatCaller: func(i interface{}) string {
				caller, _ := i.(string)
				file := stripCallerPath(caller)
				return "\033[34m" + file + "\033[0m"
			},
		},
	}
}

// buildFileWriter creates the rotating file writer, returning nil on setup failure.
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

	filename := fmt.Sprintf("%s_%s%s.log",
		cfg.LogFileName,
		time.Now().Format("02-01-2006_15-04-05"),
		suffix,
	)

	return &FileWriterWithLevel{
		Logger: &lumberjack.Logger{
			Filename:   filepath.Join(logDir, filename),
			MaxSize:    cfg.MaxFileSize,
			MaxBackups: 3,
			MaxAge:     30,
		},
		Formatted: cfg.Formatted, // pass the flag so WriteLevel can use it
	}
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
