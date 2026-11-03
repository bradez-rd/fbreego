package config

import (
	"os"
	"strings"
)

// Value returns a normalized fbreego environment variable.
func Value(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// Bool reads a boolean from a fbreego environment variable.
func Bool(name string, defaultValue bool) bool {
	switch strings.ToLower(Value(name)) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return defaultValue
	}
}

// DataDir returns the configured directory or the fbreego default.
func DataDir(name, defaultValue string) string {
	if value := Value(name); value != "" {
		return value
	}
	return defaultValue
}
