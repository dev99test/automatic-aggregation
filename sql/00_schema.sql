CREATE SCHEMA IF NOT EXISTS underpass;

CREATE TABLE IF NOT EXISTS underpass.analysis_raw_daily (
    ymd date NOT NULL,
    site_id text NOT NULL,
    device_id text NOT NULL,
    schema_version text,
    tar_filename text,
    raw_json jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ymd, site_id, device_id)
);

CREATE TABLE IF NOT EXISTS underpass.daily_issue (
    ymd date NOT NULL,
    site_id text NOT NULL,
    device_id text NOT NULL,
    sensor_type text NOT NULL,
    sensor_dir text NOT NULL,
    issue_type text NOT NULL,
    issue_count integer NOT NULL,
    samples jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ymd, site_id, device_id, sensor_type, sensor_dir, issue_type)
);

CREATE TABLE IF NOT EXISTS underpass.daily_metric (
    ymd date NOT NULL,
    site_id text NOT NULL,
    device_id text NOT NULL,
    sensor_type text NOT NULL,
    sensor_dir text NOT NULL,
    metric_key text NOT NULL,
    metric_num double precision,
    metric_text text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (ymd, site_id, device_id, sensor_type, sensor_dir, metric_key)
);

CREATE INDEX IF NOT EXISTS daily_issue_sensor_idx
    ON underpass.daily_issue (site_id, device_id, sensor_type, sensor_dir, ymd);

CREATE INDEX IF NOT EXISTS daily_metric_sensor_idx
    ON underpass.daily_metric (site_id, device_id, sensor_type, sensor_dir, ymd);
