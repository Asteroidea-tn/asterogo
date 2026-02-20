#  Env Loader



A lightweight, type-safe environment variable loader for Go applications with automatic struct field mapping using tags.

## Features

✅ Simple `env` tag-based configuration mapping  
✅ Support for multiple data types (string, int, bool, float64)  
✅ Nested struct support  
✅ Default value handling  
✅ Required and optional variables  
✅ `.env` file support (via godotenv)  
✅ Zero additional dependencies  
✅ Reflection-based automatic casting  

## Installation

```bash
go get github.com/yourusername/env_loader
```

### Requirements
- Go 1.16 or higher
- (Optional) `github.com/joho/godotenv` for `.env` file support

## Quick Start

### 1. Define Your Config Struct

Use `env` tags to map environment variables to struct fields:

```go
type AppConfig struct {
    Server struct {
        Port  int    `env:"SERVER_PORT,8080"`        // default: 8080
        Host  string `env:"SERVER_HOST,localhost"`   // default: localhost
        Debug bool   `env:"SERVER_DEBUG,false"`      // default: false
    }
}
```

### 2. Load Configuration

```go
package main

import (
    "log"
    "github.com/joho/godotenv"
)

func main() {
    var cfg AppConfig
    
    // Map env vars into the struct
    if err := LoadConfigVars(&cfg); err != nil {
        log.Fatalf("Failed to load env: %v", err)
    }

    log.Printf("Config loaded → port:%d, host:%s", cfg.Server.Port, cfg.Server.Host)
}
```

## Tag Format

The `env` tag supports two formats:

### Required Variable (No Default)
```go
Port int `env:"SERVER_PORT"`  // Error if SERVER_PORT is not set
```

### Optional Variable (With Default)
```go
Port int `env:"SERVER_PORT,8080"`  // Uses 8080 if SERVER_PORT is not set
```

## Supported Types

- **string** - Text values
- **int** - Integer values
- **bool** - Boolean values (true/false, 1/0, yes/no)
- **float64** - Floating-point numbers

## Nested Structs

Full support for nested struct fields:

```go
type AppConfig struct {
    Server struct {
        Port int `env:"SERVER_PORT,8080"`
    }
    Database struct {
        Host     string `env:"DB_HOST,localhost"`
        Port     int    `env:"DB_PORT,5432"`
        Username string `env:"DB_USER"`  // required
    }
}
```

### Nested Example with Complete Usage

```go
type AppConfig struct {
    App struct {
        Name    string `env:"APP_NAME,MyApp"`
        Version string `env:"APP_VERSION,1.0.0"`
        Debug   bool   `env:"APP_DEBUG,false"`
    }
    Server struct {
        Port     int    `env:"SERVER_PORT,8080"`
        Host     string `env:"SERVER_HOST,localhost"`
        Timeout  int    `env:"SERVER_TIMEOUT,30"`
    }
    Database struct {
        Host     string  `env:"DB_HOST,localhost"`
        Port     int     `env:"DB_PORT,5432"`
        Name     string  `env:"DB_NAME,appdb"`
        User     string  `env:"DB_USER"`        // required
        Password string  `env:"DB_PASSWORD"`    // required
        MaxConn  int     `env:"DB_MAX_CONN,10"`
    }
    Cache struct {
        Enabled bool   `env:"CACHE_ENABLED,true"`
        TTL     int    `env:"CACHE_TTL,3600"`
        Host    string `env:"CACHE_HOST,localhost"`
    }
}
```
## Best Practices

1. **Use nested structs** for better organization and readability
2. **Always set defaults** for non-critical configuration
3. **Document required variables** in your `.env` file
4. **Validate configuration** after loading (e.g., port ranges, URLs)
7. **Use uppercase names** for environment variables (convention)




## Error Handling

The loader will return an error if a required environment variable is missing:

```go
if err := LoadConfigVars(&cfg); err != nil {
    // err: missing required env variable "SERVER_PORT" (for field "Port")
    log.Fatalf("Failed to load configuration: %v", err)
}
```


## License

**Asteroidea R&D Department**  
* Author: Yassine MANAI





