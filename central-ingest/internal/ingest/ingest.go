package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Run(ctx context.Context, cfg Config) error {
	if cfg.DSN == "" && !cfg.DryRun {
		return fmt.Errorf("dsn is required unless --dry-run is set")
	}
	for _, dir := range []string{cfg.InboxDir, cfg.Processing, cfg.Processed, cfg.Failed} {
		if dir == "" {
			return fmt.Errorf("directory path missing")
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	if cfg.Loop {
		cfg.Once = false
	}
	var db *DB
	var conn *sql.DB
	var err error
	if !cfg.DryRun {
		switch strings.ToLower(cfg.PGDriver) {
		case "", "pgx":
			cfg.PGDriver = "pgx"
		case "postgres", "postgresql":
			cfg.PGDriver = "pgx"
		default:
			return fmt.Errorf("unsupported pg_driver=%q (use pgx)", cfg.PGDriver)
		}
		conn, err = sql.Open(cfg.PGDriver, cfg.DSN)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer conn.Close()
		db = NewDB(conn, cfg.PGSchema)
	}

	for {
		hadFailure, err := processBatch(ctx, cfg, db)
		if err != nil {
			return err
		}
		if cfg.Once {
			if hadFailure {
				return ExitWithFailures
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.LoopDelay):
		}
	}
}

var ExitWithFailures = errors.New("batch completed with failures")

func processBatch(ctx context.Context, cfg Config, db *DB) (bool, error) {
	files, err := filepath.Glob(filepath.Join(cfg.InboxDir, "*.tar.gz"))
	if err != nil {
		return false, fmt.Errorf("scan inbox: %w", err)
	}
	if len(files) == 0 {
		log.Printf("event=inbox_empty")
		return false, nil
	}
	sort.Strings(files)

	hadFailure := false
	for _, path := range files {
		if err := processFile(ctx, cfg, db, path); err != nil {
			log.Printf("event=file_failed file=%s error=%s", filepath.Base(path), err)
			hadFailure = true
		}
	}
	return hadFailure, nil
}

func processFile(ctx context.Context, cfg Config, db *DB, inboxPath string) error {
	base := filepath.Base(inboxPath)
	processingPath := filepath.Join(cfg.Processing, base)
	if err := os.Rename(inboxPath, processingPath); err != nil {
		return fmt.Errorf("move to processing: %w", err)
	}
	var resultErr error
	defer func() {
		if resultErr == nil {
			return
		}
		failedPath := filepath.Join(cfg.Failed, base)
		if moveErr := os.Rename(processingPath, failedPath); moveErr != nil {
			log.Printf("event=move_failed file=%s error=%s", base, moveErr)
		}
	}()

	analysisBytes, err := readAnalysisJSON(processingPath, cfg.MaxJSONBytes)
	if err != nil {
		resultErr = err
		return err
	}
	analysis, err := ParseAnalysis(analysisBytes)
	if err != nil {
		resultErr = err
		return err
	}

	siteID, deviceID, ymd, err := resolveIdentifiers(base, analysis)
	if err != nil {
		resultErr = err
		return err
	}

	if cfg.DryRun {
		log.Printf("event=dry_run file=%s site_id=%s device_id=%s ymd=%s", base, siteID, deviceID, ymd)
		inboxReturn := filepath.Join(cfg.InboxDir, base)
		if err := os.Rename(processingPath, inboxReturn); err != nil {
			resultErr = fmt.Errorf("move back to inbox: %w", err)
			return resultErr
		}
		return nil
	}

	if db == nil {
		resultErr = fmt.Errorf("database not configured")
		return resultErr
	}

	err = db.WithTx(ctx, func(tx *sql.Tx) error {
		if err := db.InsertRaw(ctx, tx, ymd, siteID, deviceID, analysis.SchemaVersion, base, analysisBytes); err != nil {
			return err
		}
		for _, sensor := range analysis.Sensors {
			if err := ingestSensor(ctx, db, tx, ymd, siteID, deviceID, sensor); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		resultErr = err
		return err
	}

	processedPath := filepath.Join(cfg.Processed, base)
	if err := os.Rename(processingPath, processedPath); err != nil {
		resultErr = fmt.Errorf("move to processed: %w", err)
		return resultErr
	}

	log.Printf("event=file_processed file=%s site_id=%s device_id=%s ymd=%s", base, siteID, deviceID, ymd)
	return nil
}

func readAnalysisJSON(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	reader := io.LimitReader(file, maxBytes+10*1024)
	return ExtractAnalysisJSON(reader, maxBytes)
}

func resolveIdentifiers(filename string, analysis Analysis) (string, string, string, error) {
	siteID, deviceID, ymd, ok := ParseFilename(filename)
	if !ok {
		siteID = strings.TrimSpace(analysis.SiteID)
		deviceID = strings.TrimSpace(analysis.DeviceID)
		if siteID == "" && analysis.Site != nil {
			siteID = strings.TrimSpace(analysis.Site.ID)
		}
		if analysis.Date != "" {
			ymd = analysis.Date
		}
	}
	if ymd == "" && analysis.Date != "" {
		ymd = analysis.Date
	}
	if siteID == "" || deviceID == "" || ymd == "" {
		return "", "", "", fmt.Errorf("missing identifiers")
	}
	return siteID, deviceID, ymd, nil
}

func ingestSensor(ctx context.Context, db *DB, tx *sql.Tx, ymd, siteID, deviceID string, sensor Sensor) error {
	for _, issue := range sensor.Issues {
		if issue.Type == "" {
			continue
		}
		count, ok := IssueCount(issue.Count)
		if !ok {
			continue
		}
		if err := db.UpsertIssue(ctx, tx, ymd, siteID, deviceID, sensor.SensorType, sensor.SensorDir, issue.Type, count, issue.Examples); err != nil {
			return err
		}
	}

	metrics := map[string]interface{}{}
	for key, val := range sensor.Metrics {
		metrics[key] = val
	}
	for key, val := range sensor.Frames {
		metricKey := fmt.Sprintf("frames_%s", key)
		if _, exists := metrics[metricKey]; !exists {
			metrics[metricKey] = val
		}
	}

	for key, val := range metrics {
		num, text := MetricValue(val)
		if num == nil && text == nil {
			continue
		}
		if err := db.UpsertMetric(ctx, tx, ymd, siteID, deviceID, sensor.SensorType, sensor.SensorDir, key, num, text); err != nil {
			return err
		}
	}
	return nil
}

func MarshalAnalysis(analysis Analysis) ([]byte, error) {
	data, err := json.Marshal(analysis)
	if err != nil {
		return nil, fmt.Errorf("marshal analysis: %w", err)
	}
	return data, nil
}
