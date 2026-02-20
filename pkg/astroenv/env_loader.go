package astroenv

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// LoadEnv reads environment variables into a struct using `env` tags.
//
// Tag format:
//
//	`env:"ENV_KEY"`           → required, error if missing
//	`env:"ENV_KEY,default"`   → optional, uses default if missing
//
// Supported types: string, int, bool, float64
// Supports nested structs.
func LoadEnvVarible(cfg interface{}) error {

	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("Warning: Could not load .env file: %v", err)
	}

	// We need a pointer to a struct to be able to set fields
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("LoadEnv: expected a pointer to a struct, got %T", cfg)
	}

	return parseStruct(v.Elem())
}

// parseStruct iterates over every field in the struct and processes its `env` tag.
// If a field is itself a nested struct, it recurses into it.
func parseStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// ── Nested struct → recurse ──────────────────────────────────────────
		if field.Kind() == reflect.Struct {
			if err := parseStruct(field); err != nil {
				return err
			}
			continue
		}

		// ── Read the `env` tag ───────────────────────────────────────────────
		tag := fieldType.Tag.Get("env")
		if tag == "" {
			continue // no env tag, skip this field
		}

		key, defaultVal, hasDefault := parseTag(tag)

		// ── Resolve the value: env var → default → error ─────────────────────
		rawVal, err := resolveValue(key, defaultVal, hasDefault, fieldType.Name)
		if err != nil {
			return err
		}

		// ── Cast and set the value into the struct field ──────────────────────
		if err := setField(field, fieldType.Name, rawVal); err != nil {
			return err
		}
	}

	return nil
}

// parseTag splits "ENV_KEY,default_value" into its parts.
// Returns: key, defaultValue, hasDefault
func parseTag(tag string) (string, string, bool) {
	parts := strings.SplitN(tag, ",", 2)
	key := strings.TrimSpace(parts[0])

	if len(parts) == 2 {
		return key, strings.TrimSpace(parts[1]), true
	}

	return key, "", false
}

// resolveValue looks up the env var. Falls back to default. Errors if required and missing.
func resolveValue(key, defaultVal string, hasDefault bool, fieldName string) (string, error) {
	if val := os.Getenv(key); val != "" {
		return val, nil
	}

	if hasDefault {
		return defaultVal, nil
	}

	return "", fmt.Errorf("missing required env variable %q (for field %q)", key, fieldName)
}

// setField converts the raw string value to the correct type and sets it on the struct field.
func setField(field reflect.Value, fieldName, rawVal string) error {
	switch field.Kind() {

	case reflect.String:
		field.SetString(rawVal)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(rawVal, 10, 64)
		if err != nil {
			return fmt.Errorf("field %q: cannot parse %q as int: %w", fieldName, rawVal, err)
		}
		field.SetInt(n)

	case reflect.Bool:
		b, err := strconv.ParseBool(rawVal)
		if err != nil {
			return fmt.Errorf("field %q: cannot parse %q as bool (use true/false/1/0): %w", fieldName, rawVal, err)
		}
		field.SetBool(b)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(rawVal, 64)
		if err != nil {
			return fmt.Errorf("field %q: cannot parse %q as float: %w", fieldName, rawVal, err)
		}
		field.SetFloat(f)

	default:
		return fmt.Errorf("field %q: unsupported type %s", fieldName, field.Kind())
	}

	return nil
}
