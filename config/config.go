package config

import (
	"os"
	"strconv"
)

type Config struct {
	// Collector config
	ServerPort     int
	OutputDir      string
	TraceFileName  string
	MetricFileName string
	LogFileName    string

	// Aggregator config
	AggregatorEnabled  bool
	AggregatorPort     int
	DBPath             string
	ProcessingInterval int
}

func Load() *Config {
	return &Config{
		ServerPort:         getEnvAsInt("OTIS_PORT", 4318),
		OutputDir:          getEnv("OTIS_OUTPUT_DIR", "./data"),
		TraceFileName:      getEnv("OTIS_TRACE_FILE", "traces.jsonl"),
		MetricFileName:     getEnv("OTIS_METRIC_FILE", "metrics.jsonl"),
		LogFileName:        getEnv("OTIS_LOG_FILE", "logs.jsonl"),
		AggregatorEnabled:  getEnvAsBool("OTIS_AGGREGATOR_ENABLED", true),
		AggregatorPort:     getEnvAsInt("OTIS_AGGREGATOR_PORT", 8080),
		DBPath:             getEnv("OTIS_DB_PATH", "./db/otis.db"),
		ProcessingInterval: getEnvAsInt("OTIS_PROCESSING_INTERVAL", 5),
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

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
