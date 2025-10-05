package configloader

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// OverrideString overrides a string field if the environment variable is set.
func OverrideString(field *string, env string) {
	if val := os.Getenv(env); val != "" {
		*field = val
	}
}

// OverrideInt overrides an int field if the environment variable is set.
func OverrideInt(field *int, env string) {
	if val := os.Getenv(env); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			*field = i
		}
	}
}

// OverrideBool overrides a bool field if the environment variable is set.
func OverrideBool(field *bool, env string) {
	if val := os.Getenv(env); val != "" {
		switch val {
		case "1", "true", "TRUE", "True":
			*field = true
		case "0", "false", "FALSE", "False":
			*field = false
		}
	}
}

// OverrideStringSlice overrides a []string field if the environment variable is set.
// The variable must be a comma-separated list (e.g., "node-1,node-2,node-3").
func OverrideStringSlice(field *[]string, env string) {
	if val := os.Getenv(env); val != "" {
		parts := strings.Split(val, ",")
		trimmed := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				trimmed = append(trimmed, p)
			}
		}
		*field = trimmed
	}
}

// OverrideFloat overrides a float64 field if the environment variable is set.
func OverrideFloat(field *float64, env string) {
	if val := os.Getenv(env); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			*field = f
		}
	}
}

// OverrideDuration overrides a time.Duration field if the environment variable is set.
func OverrideDuration(field *time.Duration, env string) {
	if val := os.Getenv(env); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			*field = d
		}
	}
}

// OverrideInt64 overrides an int64 field if the environment variable is set.
// The environment variable must contain a valid integer (e.g., "1024").
func OverrideInt64(field *int64, env string) {
	if val := os.Getenv(env); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			*field = i
		}
	}
}
