Here's the updated README with the logging options documentation added:

---

# ASTRO LOG

`astrolog` is the logging package for this project. It wraps [`zerolog`](https://github.com/rs/zerolog) and optionally writes logs to rotating files with [`lumberjack`](https://github.com/natefinch/lumberjack).

## What it does

`InitLogger` configures the global `zerolog` logger so you can log to the console, to files, or to both at the same time.

The package supports two file rotation modes:

- `RotationDaily`: one file per day, reused if the app restarts on the same day.
- `RotationPerRun`: a new file is created for every process start.

When file logging is enabled, the package also:

- creates a `./logs` directory if needed
- removes old files by age if `MaxAgeDays` is set
- removes old files by count if `MaxLogFiles` is set
- writes a banner when a run starts, and a restart banner when a daily file already exists

---

## Import

```go
import "github.com/Asteroidea-tn/asterogo/pkg/astrolog"
```

```bash
go get github.com/Asteroidea-tn/asterogo/pkg/astrolog
```

If your project is inside this module, you can use the same import path from any Go file in the module.

---

## Quick Start

```go
package main

import "github.com/Asteroidea-tn/asterogo/pkg/astrolog"

func main() {
    cfg := astrolog.CofigLogger{
        LogLevel:     "info",
        LogToFile:    false,
        LogFileName:  "app",
        Formatted:    false,
        RotationMode: astrolog.RotationDaily,
        MaxFileSize:  10,
        MaxLogFiles:  5,
        MaxAgeDays:   30,
    }

    astrolog.InitLogger(cfg)
}
```

---

## Configuration

Use `astrolog.CofigLogger` to control the logger.

| Field | Type | Description |
| --- | --- | --- |
| `LogLevel` | `string` | Sets the global zerolog level. Common values: `trace`, `debug`, `info`, `warn`, `error`. Invalid values fall back to `info`. |
| `LogToFile` | `bool` | Enables file logging under `./logs`. |
| `LogFileName` | `string` | Base name of the generated log file. Example: `app`. |
| `Formatted` | `bool` | `true` writes raw JSON everywhere. `false` writes pretty console/file output. |
| `RotationMode` | `astrolog.RotationMode` | Chooses the file strategy (`RotationDaily` or `RotationPerRun`). |
| `MaxFileSize` | `int` | Maximum size in MB before `lumberjack` rotates the file. `0` uses the default `100 MB`. |
| `MaxLogFiles` | `int` | Maximum number of `.log` files kept in `./logs`. `0` disables package-level count cleanup. |
| `MaxAgeDays` | `int` | Deletes log files older than this number of days on startup. `0` disables age cleanup. |

### Defaults

If you pass an empty or partial `CofigLogger`, missing fields are filled in automatically:

| Field | Default Value |
| --- | --- |
| `LogLevel` | `"info"` |
| `LogFileName` | `"app"` |
| `RotationMode` | `RotationPerRun` |
| `MaxFileSize` | `5` MB |
| `MaxLogFiles` | `30` |
| `MaxAgeDays` | `30` |

```go
// Minimal — everything else is defaulted.
astrolog.InitLogger(astrolog.CofigLogger{
    LogToFile: true,
})

// Or start from the defaults and override what you need.
cfg := astrolog.DefaultConfig()
cfg.LogLevel = "debug"
astrolog.InitLogger(cfg)
```

---

## Rotation Modes

### `RotationDaily`

Creates files using the pattern:

```text
<LogFileName>_DD-MM-YYYY.log
```

If the file already exists (e.g. after a container restart on the same day), new logs are appended and a restart banner is written first.

### `RotationPerRun`

Creates files using the pattern:

```text
<LogFileName>_DD-MM-YYYY_HHMMSS.log
```

Every process start gets its own file. No two runs share a file.

---

## Logging Options

### Log Levels

Set `LogLevel` to control which entries are emitted. Lower levels include all levels above them.

| Level | Constant | When to use |
| --- | --- | --- |
| `trace` | `zerolog.TraceLevel` | Very fine-grained steps, usually only during development. |
| `debug` | `zerolog.DebugLevel` | Diagnostic info useful during development and testing. |
| `info` | `zerolog.InfoLevel` | Normal operational events. Default level. |
| `warn` | `zerolog.WarnLevel` | Unexpected states that do not stop execution. |
| `error` | `zerolog.ErrorLevel` | Failures that need attention but allow the process to continue. |
| `fatal` | `zerolog.FatalLevel` | Critical failures — logs the entry then calls `os.Exit(1)`. |
| `panic` | `zerolog.PanicLevel` | Logs the entry then panics. |

```go
// Only emit warn and above — debug and info are silenced.
astrolog.InitLogger(astrolog.CofigLogger{
    LogLevel: "warn",
})

// Change the level at runtime without reinitializing the logger.
astrolog.UpdateLogLevel("debug")
```

### Output Format

`Formatted` controls how entries look, both on the console and in files.

| `Formatted` | Console | File |
| --- | --- | --- |
| `false` (default) | Pretty, colorized, human-readable | Pretty, pipe-delimited plain text |
| `true` | Raw JSON | Raw JSON |

Pretty file output follows this layout:

```text
2006-01-02 15:04:05.0 | info  | server:42             | server started | port=8080
```

Fields are: `timestamp | level | caller | message | extra key=value pairs`.

### Console vs File

You can run console-only, file-only, or both simultaneously.

```go
// Console only (default).
astrolog.InitLogger(astrolog.CofigLogger{
    LogToFile: false,
})

// File only — suppress console output by enabling Formatted
// and redirecting stderr, or simply leave LogToFile true
// and redirect the process stderr at the OS level.
astrolog.InitLogger(astrolog.CofigLogger{
    LogToFile:   true,
    LogFileName: "app",
})

// Both at once.
astrolog.InitLogger(astrolog.CofigLogger{
    LogToFile:   true,
    LogFileName: "app",
    LogLevel:    "debug",
})
```

### Structured Fields

Because `astrolog` uses zerolog under the hood, you can attach typed key-value pairs to any log entry using the standard zerolog chaining API.

```go
import "github.com/rs/zerolog/log"

log.Info().
    Str("service", "payment").
    Int("attempt", 3).
    Dur("elapsed", duration).
    Msg("charge completed")

log.Error().
    Err(err).
    Str("order_id", id).
    Msg("charge failed")

log.Debug().
    Interface("payload", req).
    Msg("incoming request")
```

Extra fields appear at the end of pretty-formatted lines as `key=value` pairs, and as normal JSON fields in JSON mode.

### Runtime Level Changes

You can change the log level at any point without reinitializing the logger. This is safe to call from multiple goroutines.

```go
// Raise verbosity temporarily for a debug session.
astrolog.UpdateLogLevel("trace")

// Return to normal.
astrolog.UpdateLogLevel("info")
```

### Accessing the Logger Directly

If you need a local `zerolog.Logger` value — for example, to attach a fixed field to all entries from a specific component — use `GetLogger`:

```go
log := log.GetLogger().With().Str("TransctionID", transacstion.ID).Logger()

log.Info().Msg("worker started") // the transaction id will passed here automaticly without pass it again
log.Warn().Int("queue_depth", n).Msg("queue backing up") // the transaction id will passed here automaticly without pass it again
```

This does not affect the global logger; it creates a derived copy.

---

## Exported API

### `InitLogger(cfg CofigLogger)`

Initializes the global logger. Call once at application startup, before emitting any logs.

### `DefaultConfig() CofigLogger`

Returns a `CofigLogger` with all fields set to their default values. Use it as a starting point when you only need to override one or two fields.

### `GetLogger() zerolog.Logger`

Returns the current global `zerolog.Logger`. Use when you need a local or derived logger instance.

### `UpdateLogLevel(level string)`

Changes the global log level at runtime. Falls back to `info` if the string cannot be parsed.

### `RotationMode`

The rotation mode type. Available constants: `RotationDaily`, `RotationPerRun`.

### `ConsoleWriterWithLevel`

A wrapper around `zerolog.ConsoleWriter` that implements `zerolog.LevelWriter`.

### `FileWriterWithLevel`

A wrapper around `lumberjack.Logger` that writes either raw JSON or pretty-formatted lines depending on `Formatted`.

---

## Logging Behavior

When `Formatted` is `false`, file logs are rendered as readable pipe-delimited lines:

```text
timestamp | level | caller | message | key=value ...
```

When `Formatted` is `true`, file output is raw JSON, one object per line.

The caller path is always shortened to `filename:line` to keep log lines compact.

---

## Example: Full Setup

```go
package main

import (
    "github.com/Asteroidea-tn/asterogo/pkg/astrolog"
    "github.com/rs/zerolog/log"
)

func main() {
    astrolog.InitLogger(astrolog.CofigLogger{
        LogLevel:     "debug",
        LogToFile:    true,
        LogFileName:  "app",
        Formatted:    false,
        RotationMode: astrolog.RotationDaily,
        MaxFileSize:  50,
        MaxLogFiles:  7,
        MaxAgeDays:   14,
    })

    log.Info().Str("env", "production").Msg("service started")
    log.Debug().Int("workers", 4).Msg("pool initialized")
    log.Warn().Str("config", "missing key").Msg("falling back to default")
    log.Error().Err(err).Msg("connection failed")
}
```

---

## Notes

- Logs are written to `./logs` relative to the working directory.
- If file creation fails, the logger still starts with the available writers (console output is preserved).
- The type name `CofigLogger` is spelled as it appears in the source code.

---

## License

**Asteroidea R&D Department**  
Author: Yassine MANAI