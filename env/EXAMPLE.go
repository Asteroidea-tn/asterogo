package env

import (
	"log"
)

// ───────────────────────────────────────────
// EXAMPMLE USAGE ────────────────────────────
// ───────────────────────────────────────────

/*
.env FILE example:

SERVER_HOST=0.0.0.0
SERVER_DEBUG=true

*/

type ConfigExample struct {
	// normal attrb
	DEBUG_LEVEL string `env:"DEBUG_LEVEL,info"` // default "info" - if debug level is not set, it will default to "info"
	Port        int    `env:"PORT,3000"`

	// nested struct
	Server struct {
		Port int `env:"SERVER_PORT,8080"`
	}
}

func main() {
	var cfg ConfigExample
	if err := LoadEnvVarible(&cfg); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Now cfg is populated with environment variables or defaults
	log.Printf("Config: %+v\n", cfg)
}
