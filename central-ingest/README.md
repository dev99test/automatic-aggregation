# Central Ingest Pipeline

This program ingests daily analysis tarballs on the Central server and writes results into PostgreSQL for Grafana.

## Directory layout

```
central-ingest/
  main.go
  internal/ingest/
sql/
  00_schema.sql
  01_views.sql
  02_roles.sql
deploy/systemd/
  ingest-tar-to-pg.service
  ingest-tar-to-pg.timer
```

## Build

```bash
go build ./...
```

## Run once

```bash
./central-ingest --dsn "postgres://user:pass@localhost:5432/dbname" \
  --inbox /var/lib/central-ingest/inbox \
  --processing /var/lib/central-ingest/processing \
  --processed /var/lib/central-ingest/processed \
  --failed /var/lib/central-ingest/failed
```

### Dry run

```bash
./central-ingest --dry-run --inbox /var/lib/central-ingest/inbox
```

## Configuration file

You can provide a JSON config file with the same settings as flags.

Example `/etc/central-ingest/config.json`:

```json
{
  "dsn": "postgres://ingest:password@localhost:5432/underpass",
  "pg_driver": "postgres",
  "pg_schema": "underpass",
  "inbox": "/var/lib/central-ingest/inbox",
  "processing": "/var/lib/central-ingest/processing",
  "processed": "/var/lib/central-ingest/processed",
  "failed": "/var/lib/central-ingest/failed",
  "max_json_bytes": 52428800,
  "once": true,
  "dry_run": false
}
```

## Database setup

Apply the SQL files in order:

```bash
psql "$DSN" -f sql/00_schema.sql
psql "$DSN" -f sql/01_views.sql
psql "$DSN" -f sql/02_roles.sql
```

## Systemd

1. Copy the unit files:

```bash
sudo cp deploy/systemd/ingest-tar-to-pg.service /etc/systemd/system/
sudo cp deploy/systemd/ingest-tar-to-pg.timer /etc/systemd/system/
```

2. Create the working directories and config:

```bash
sudo useradd --system --home /var/lib/central-ingest --shell /usr/sbin/nologin central-ingest
sudo mkdir -p /var/lib/central-ingest/{inbox,processing,processed,failed}
sudo chown -R central-ingest:central-ingest /var/lib/central-ingest
sudo mkdir -p /etc/central-ingest
sudo tee /etc/central-ingest/config.json > /dev/null <<'CONFIG'
{
  "dsn": "postgres://ingest:password@localhost:5432/underpass",
  "pg_driver": "postgres",
  "pg_schema": "underpass",
  "inbox": "/var/lib/central-ingest/inbox",
  "processing": "/var/lib/central-ingest/processing",
  "processed": "/var/lib/central-ingest/processed",
  "failed": "/var/lib/central-ingest/failed",
  "max_json_bytes": 52428800,
  "once": true,
  "dry_run": false
}
CONFIG
sudo chown -R central-ingest:central-ingest /etc/central-ingest
```

3. Enable the timer:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ingest-tar-to-pg.timer
```

## Verify ingestion

After a run, check rows:

```bash
psql "$DSN" -c "SELECT ymd, site_id, device_id FROM underpass.analysis_raw_daily ORDER BY ymd DESC LIMIT 5;"
```

## Troubleshooting

- **Permissions**: Ensure the `central-ingest` user can read the inbox and write to processing/processed/failed directories.
- **DSN errors**: Validate the DSN and that the ingest user has INSERT/UPDATE rights on the `underpass` schema.
- **Driver errors**: The `pg_driver` setting must match a registered Go SQL driver (for example, `postgres` from lib/pq or `pgx` from pgx stdlib).
- **Tar name mismatch**: Filenames should match `site_device_YYYYMMDD.tar.gz`. If not, the program falls back to `analysis.json` fields.
- **Missing analysis.json**: The ingest program will move tarballs without `analysis.json` to `failed/`.

## 실행 및 자동화 운영 가이드 (Ubuntu 22.04 + systemd)

아래 내용은 운영자가 그대로 따라 할 수 있도록 구성된 “실행/자동화” 절차입니다.

### 1) 디렉터리 구성 (권장 경로)

```bash
mkdir -p /home/eum/ingest/{inbox,processing,processed,failed}
```

- **/home/eum/ingest/inbox**: SCP로 tar.gz가 업로드되는 수신함
- **/home/eum/ingest/processing**: 처리 중인 tar.gz가 이동되는 임시 영역
- **/home/eum/ingest/processed**: 처리 성공한 tar.gz 보관
- **/home/eum/ingest/failed**: 처리 실패한 tar.gz 보관(재처리용)

### 2) 실행 파일 준비

바이너리는 예시로 아래 경로에 둡니다.

```bash
sudo install -m 0755 /path/to/central-ingest /usr/local/bin/ingest_tar_to_pg
```

실행 권한을 확인합니다.

```bash
ls -l /usr/local/bin/ingest_tar_to_pg
```

### 3) PostgreSQL 준비 (간단 확인)

DSN 예시(sslmode=disable 포함):

```bash
postgres://ingest_user:password@127.0.0.1:5432/underpass?sslmode=disable
```

- **ingest 전용 계정**은 INSERT/UPDATE 권한이 필요합니다.
- **grafana_ro 계정**은 SELECT 권한만 부여하는 것을 권장합니다.

### 4) 수동 실행 (1회 실행)

```bash
/usr/local/bin/ingest_tar_to_pg \
  --dsn "postgres://ingest_user:password@127.0.0.1:5432/underpass?sslmode=disable" \
  --pg-schema underpass \
  --inbox /home/eum/ingest/inbox \
  --processing /home/eum/ingest/processing \
  --processed /home/eum/ingest/processed \
  --failed /home/eum/ingest/failed
```

- 성공 시 tar.gz는 **processed** 폴더로 이동합니다.
- 실패 시 tar.gz는 **failed** 폴더로 이동합니다.

로그 확인:

```bash
# 표준 출력(터미널)
/usr/local/bin/ingest_tar_to_pg --dry-run --inbox /home/eum/ingest/inbox

# systemd 로그(서비스로 실행한 경우)
journalctl -u ingest-tar-to-pg.service -e
```

### 5) 자동 실행 (systemd timer)

`/etc/systemd/system/ingest-tar-to-pg.service` 예시:

```ini
[Unit]
Description=Central ingest pipeline for analysis tarballs
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=eum
Group=eum
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
ExecStart=/usr/local/bin/ingest_tar_to_pg \
  --dsn "postgres://ingest_user:password@127.0.0.1:5432/underpass?sslmode=disable" \
  --pg-schema underpass \
  --inbox /home/eum/ingest/inbox \
  --processing /home/eum/ingest/processing \
  --processed /home/eum/ingest/processed \
  --failed /home/eum/ingest/failed
WorkingDirectory=/home/eum/ingest

[Install]
WantedBy=multi-user.target
```

`/etc/systemd/system/ingest-tar-to-pg.timer` 예시:

```ini
[Unit]
Description=Run central ingest every 5 minutes

[Timer]
OnCalendar=*:0/5
Persistent=true
Unit=ingest-tar-to-pg.service

[Install]
WantedBy=timers.target
```

등록 및 실행:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ingest-tar-to-pg.timer
```

타이머 등록 확인 및 로그:

```bash
systemctl list-timers --all | grep ingest-tar-to-pg
journalctl -u ingest-tar-to-pg.service -e
```

### 6) 장애/트러블슈팅 체크리스트

- **inbox에 파일이 안 들어옴**: SCP 경로/권한 확인
- **analysis.json 없음**: tar.gz 내부에 파일이 누락된 경우 → failed로 이동
- **파일명 패턴 불일치**: `site_device_YYYYMMDD.tar.gz` 형식 확인
- **DB 접속 실패**: DSN/계정 권한/방화벽 확인
- **중복 적재**: 동일 (ymd, site_id, device_id)는 Upsert로 갱신됨

### 7) 보안/운영 팁 (관공서 환경 고려)

- 외부 통신 없이 로컬 환경에서 동작합니다.
- Grafana는 **읽기 전용 계정**으로 접속하도록 구성합니다.
- failed 폴더 재처리: 파일을 **inbox**로 이동 후 타이머가 처리하도록 대기하거나 수동 실행합니다.
