package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort     int
	OutputDir      string
	TraceFileName  string
	MetricFileName string
	LogFileName    string
}

func Load() *Config {
	return &Config{
		ServerPort:     getEnvAsInt("OTIS_PORT", 4318),
		OutputDir:      getEnv("OTIS_OUTPUT_DIR", "./data"),
		TraceFileName:  getEnv("OTIS_TRACE_FILE", "traces.jsonl"),
		MetricFileName: getEnv("OTIS_METRIC_FILE", "metrics.jsonl"),
		LogFileName:    getEnv("OTIS_LOG_FILE", "logs.jsonl"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
