DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'grafana_readonly') THEN
        CREATE ROLE grafana_readonly NOLOGIN;
    END IF;
END $$;

GRANT USAGE ON SCHEMA underpass TO grafana_readonly;
GRANT SELECT ON underpass.analysis_raw_daily TO grafana_readonly;
GRANT SELECT ON underpass.daily_issue TO grafana_readonly;
GRANT SELECT ON underpass.daily_metric TO grafana_readonly;
GRANT SELECT ON underpass.v_daily_issue_timeseries TO grafana_readonly;
GRANT SELECT ON underpass.v_daily_issue_by_sensor TO grafana_readonly;
GRANT SELECT ON underpass.v_wls_waterlevel_trend TO grafana_readonly;
GRANT SELECT ON underpass.v_frames_trend TO grafana_readonly;

-- Create a dedicated ingest role separately with INSERT/UPDATE rights as needed.
