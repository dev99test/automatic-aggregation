CREATE OR REPLACE VIEW underpass.v_daily_issue_timeseries AS
SELECT
    ymd,
    site_id,
    device_id,
    sensor_type,
    sensor_dir,
    issue_type,
    issue_count
FROM underpass.daily_issue;

CREATE OR REPLACE VIEW underpass.v_daily_issue_by_sensor AS
SELECT
    ymd,
    site_id,
    device_id,
    sensor_type,
    sensor_dir,
    SUM(issue_count) AS total_issue_count
FROM underpass.daily_issue
GROUP BY ymd, site_id, device_id, sensor_type, sensor_dir;

CREATE OR REPLACE VIEW underpass.v_wls_waterlevel_trend AS
SELECT
    ymd,
    site_id,
    device_id,
    sensor_type,
    sensor_dir,
    metric_key,
    metric_num
FROM underpass.daily_metric
WHERE metric_key IN ('waterlevel_cm_min', 'waterlevel_cm_max', 'waterlevel_cm_last');

CREATE OR REPLACE VIEW underpass.v_frames_trend AS
SELECT
    ymd,
    site_id,
    device_id,
    sensor_type,
    sensor_dir,
    metric_key,
    metric_num
FROM underpass.daily_metric
WHERE metric_key IN ('frames_snd', 'frames_rcv');
