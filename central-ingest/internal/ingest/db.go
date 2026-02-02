package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type DB struct {
	conn    *sql.DB
	schema  string
	queries dbQueries
}

type dbQueries struct {
	insertRaw    string
	upsertIssue  string
	upsertMetric string
}

func NewDB(conn *sql.DB, schema string) *DB {
	return &DB{
		conn:   conn,
		schema: schema,
		queries: dbQueries{
			insertRaw: fmt.Sprintf(`INSERT INTO %s.analysis_raw_daily
				(ymd, site_id, device_id, schema_version, tar_filename, raw_json, received_at)
				VALUES ($1, $2, $3, $4, $5, $6, now())
				ON CONFLICT (ymd, site_id, device_id)
				DO UPDATE SET raw_json = EXCLUDED.raw_json, received_at = now(), schema_version = EXCLUDED.schema_version, tar_filename = EXCLUDED.tar_filename`, schema),
			upsertIssue: fmt.Sprintf(`INSERT INTO %s.daily_issue
				(ymd, site_id, device_id, sensor_type, sensor_dir, issue_type, issue_count, samples, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
				ON CONFLICT (ymd, site_id, device_id, sensor_type, sensor_dir, issue_type)
				DO UPDATE SET issue_count = EXCLUDED.issue_count, samples = EXCLUDED.samples, updated_at = now()`, schema),
			upsertMetric: fmt.Sprintf(`INSERT INTO %s.daily_metric
				(ymd, site_id, device_id, sensor_type, sensor_dir, metric_key, metric_num, metric_text, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
				ON CONFLICT (ymd, site_id, device_id, sensor_type, sensor_dir, metric_key)
				DO UPDATE SET metric_num = EXCLUDED.metric_num, metric_text = EXCLUDED.metric_text, updated_at = now()`, schema),
		},
	}
}

func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback: %v (original error: %w)", rollbackErr, err)
		}
		return err
	}
	return tx.Commit()
}

func (db *DB) InsertRaw(ctx context.Context, tx *sql.Tx, ymd, siteID, deviceID, schemaVersion, tarFilename string, rawJSON []byte) error {
	return exec(ctx, tx, db.queries.insertRaw, ymd, siteID, deviceID, schemaVersion, tarFilename, rawJSON)
}

func (db *DB) UpsertIssue(ctx context.Context, tx *sql.Tx, ymd, siteID, deviceID, sensorType, sensorDir, issueType string, count int64, samples interface{}) error {
	var sampleJSON []byte
	if samples != nil {
		var err error
		sampleJSON, err = json.Marshal(samples)
		if err != nil {
			return fmt.Errorf("marshal issue samples: %w", err)
		}
	}
	return exec(ctx, tx, db.queries.upsertIssue, ymd, siteID, deviceID, sensorType, sensorDir, issueType, count, sampleJSON)
}

func (db *DB) UpsertMetric(ctx context.Context, tx *sql.Tx, ymd, siteID, deviceID, sensorType, sensorDir, metricKey string, num *float64, text *string) error {
	return exec(ctx, tx, db.queries.upsertMetric, ymd, siteID, deviceID, sensorType, sensorDir, metricKey, num, text)
}

func exec(ctx context.Context, tx *sql.Tx, query string, args ...interface{}) error {
	_, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec query: %w", err)
	}
	return nil
}
