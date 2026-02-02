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

## Usage (간단 사용법)

1. inbox 디렉터리에 tar.gz가 도착했는지 확인합니다.
2. 아래 명령으로 1회 처리합니다.

```bash
./central-ingest \
  --dsn "postgres://user:pass@localhost:5432/dbname?sslmode=disable" \
  --pg-schema underpass \
  --inbox /var/lib/central-ingest/inbox \
  --processing /var/lib/central-ingest/processing \
  --processed /var/lib/central-ingest/processed \
  --failed /var/lib/central-ingest/failed
```

3. 처리 결과는 processed/failed 폴더 이동으로 확인합니다.

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

## 자동적재 프로그램 README – 사용 방법(운영 가이드)

아래 내용은 **운영자가 그대로 따라 할 수 있는 실무 중심 안내서**입니다. (Ubuntu 22.04, systemd, PostgreSQL, Grafana OSS 기준)

### 1. 디렉터리 구조

권장 경로 예시:

```
/home/eum/ingest/
  ├─ inbox/        (SCP로 tar.gz 수신)
  ├─ processing/   (처리 중)
  ├─ processed/    (처리 성공)
  ├─ failed/       (처리 실패)
```

디렉터리 생성:

```bash
mkdir -p /home/eum/ingest/{inbox,processing,processed,failed}
```

각 폴더 역할:

- **inbox**: 현장 PC에서 SCP로 전달된 tar.gz 수신
- **processing**: 처리 중인 파일이 잠시 이동되는 위치
- **processed**: 정상 처리 완료 파일 보관
- **failed**: 처리 실패 파일 보관(재처리 대상)

### 2. 실행 파일 준비

실행 파일 위치 예:

```bash
sudo install -m 0755 /path/to/central-ingest /usr/local/bin/ingest_tar_to_pg
```

권한 확인:

```bash
ls -l /usr/local/bin/ingest_tar_to_pg
```

실행 파일 역할: `tar.gz`를 읽어 `analysis.json`을 추출하고, PostgreSQL에 **자동 적재(upsert)** 합니다.

### 3. PostgreSQL 준비 사항

DB에는 사전에 스키마/테이블이 생성되어 있어야 합니다 (`sql/00_schema.sql` 적용).

DSN 예시:

```bash
postgres://ingest_user:password@127.0.0.1:5432/underpass?sslmode=disable
```

권장 계정 분리 이유:

- **ingest 전용 계정**: INSERT/UPDATE 권한이 필요 (적재 전용)
- **grafana 전용 계정**: SELECT만 허용 (시각화 전용)

운영 분리를 통해 **권한 최소화** 및 **보안 사고 영향 범위 축소**가 가능합니다.

### 4. 수동 실행 방법 (1회 실행)

운영자가 직접 1회 실행하는 명령 예시:

```bash
/usr/local/bin/ingest_tar_to_pg \
  --dsn "postgres://ingest_user:password@127.0.0.1:5432/underpass?sslmode=disable" \
  --pg-schema underpass \
  --inbox /home/eum/ingest/inbox \
  --processing /home/eum/ingest/processing \
  --processed /home/eum/ingest/processed \
  --failed /home/eum/ingest/failed
```

정상 처리 시:
- tar.gz는 **processed**로 이동

실패 시:
- tar.gz는 **failed**로 이동

표준 출력 로그 확인:

```bash
/usr/local/bin/ingest_tar_to_pg --dry-run --inbox /home/eum/ingest/inbox
```

### 5. 자동 실행 설정 (systemd)

#### 5-1. service 파일 설명

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

- **Type=oneshot**: 파일 일괄 처리 후 종료하는 방식
- **User 지정**: 시스템 권한 최소화 (운영 계정으로 실행)
- **PATH 지정**: 환경 PATH 누락으로 인한 실행 실패 방지

#### 5-2. timer 파일 설명

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

- **OnCalendar=*:0/5**: 5분마다 실행
- **Persistent=true**: 서버 재부팅 후 누락된 실행분을 보완

#### 5-3. 등록 및 확인

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ingest-tar-to-pg.timer
```

등록 확인:

```bash
systemctl list-timers --all | grep ingest-tar-to-pg
```

로그 확인:

```bash
journalctl -u ingest-tar-to-pg.service -e
```

### 6. 운영 중 확인 포인트

- **inbox에 tar.gz가 들어오는지** 확인
- **processed/failed 이동 여부** 확인
- **DB 적재 확인 SQL**:

```bash
psql "$DSN" -c "SELECT ymd, site_id, device_id FROM underpass.analysis_raw_daily ORDER BY ymd DESC LIMIT 5;"
```

Grafana는 DB의 View를 읽어 **대시보드로 시각화**합니다.

### 7. 장애 및 트러블슈팅

- **tar.gz가 처리되지 않음**: 파일 권한/디렉터리 경로 확인
- **analysis.json 없음**: tar.gz 내부에 파일 누락 → failed 이동
- **파일명 규칙 불일치**: `site_device_YYYYMMDD.tar.gz` 형식 확인
- **DB 접속 실패**: DSN/계정 권한/방화벽 확인
- **재처리 방법**: failed → inbox로 이동 후 타이머 실행

```bash
mv /home/eum/ingest/failed/*.tar.gz /home/eum/ingest/inbox/
```

### 8. 보안/관공서 운영 유의사항

- 외부 통신 없이 로컬 환경에서 동작
- Grafana는 **읽기 전용 계정**으로 접근
- tar.gz 및 JSON은 증빙 자료로 보관 가능
- 자동화 로그는 `journalctl`로 추적 가능
