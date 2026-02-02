package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	DSN          string        `json:"dsn"`
	PGDriver     string        `json:"pg_driver"`
	PGSchema     string        `json:"pg_schema"`
	InboxDir     string        `json:"inbox"`
	Processing   string        `json:"processing"`
	Processed    string        `json:"processed"`
	Failed       string        `json:"failed"`
	MaxJSONBytes int64         `json:"max_json_bytes"`
	DryRun       bool          `json:"dry_run"`
	Once         bool          `json:"once"`
	Loop         bool          `json:"loop"`
	LoopDelay    time.Duration `json:"loop_delay"`
}

func DefaultConfig() Config {
	return Config{
		PGDriver:     "postgres",
		PGSchema:     "underpass",
		InboxDir:     "/var/lib/central-ingest/inbox",
		Processing:   "/var/lib/central-ingest/processing",
		Processed:    "/var/lib/central-ingest/processed",
		Failed:       "/var/lib/central-ingest/failed",
		MaxJSONBytes: 50 * 1024 * 1024,
		Once:         true,
		LoopDelay:    5 * time.Minute,
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.PGSchema == "" {
		cfg.PGSchema = "underpass"
	}
	if cfg.PGDriver == "" {
		cfg.PGDriver = "postgres"
	}
	return cfg, nil
}
