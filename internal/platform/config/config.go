// Package config carga la configuración desde el entorno.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config agrupa los parámetros del servicio.
type Config struct {
	Env             string
	HTTPPort        string
	SearchURL       string
	SearchTimeout   time.Duration
	CacheTTL        time.Duration
	LogLevel        string
	ShutdownTimeout time.Duration
}

// Load lee el entorno con valores por defecto para desarrollo.
func Load() Config {
	return Config{
		Env:             get("APP_ENV", "local"),
		HTTPPort:        get("HTTP_PORT", "8082"),
		SearchURL:       get("SEARCH_URL", "http://localhost:9200"),
		SearchTimeout:   time.Duration(getInt("SEARCH_TIMEOUT_MS", 3000)) * time.Millisecond,
		CacheTTL:        time.Duration(getInt("CACHE_TTL_SECONDS", 30)) * time.Second,
		LogLevel:        get("LOG_LEVEL", "info"),
		ShutdownTimeout: time.Duration(getInt("SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second,
	}
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
