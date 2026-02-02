package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"automatic-aggregation/central-ingest/internal/ingest"
)

type stringFlag struct {
	set   bool
	value string
}

func (s *stringFlag) String() string { return s.value }
func (s *stringFlag) Set(val string) error {
	s.set = true
	s.value = val
	return nil
}

type boolFlag struct {
	set   bool
	value bool
}

func (b *boolFlag) String() string { return fmt.Sprintf("%t", b.value) }
func (b *boolFlag) Set(val string) error {
	parsed, err := parseBool(val)
	if err != nil {
		return err
	}
	b.set = true
	b.value = parsed
	return nil
}

type int64Flag struct {
	set   bool
	value int64
}

func (i *int64Flag) String() string { return fmt.Sprintf("%d", i.value) }
func (i *int64Flag) Set(val string) error {
	var parsed int64
	_, err := fmt.Sscanf(val, "%d", &parsed)
	if err != nil {
		return err
	}
	i.set = true
	i.value = parsed
	return nil
}

func parseBool(val string) (bool, error) {
	switch val {
	case "true", "1", "t", "yes", "y":
		return true, nil
	case "false", "0", "f", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool: %s", val)
	}
}

func main() {
	cfg := ingest.DefaultConfig()

	configPath := flag.String("config", "", "optional JSON config file")
	var dsnFlag stringFlag
	dsnFlag.value = cfg.DSN
	flag.Var(&dsnFlag, "dsn", "PostgreSQL DSN")

	var schemaFlag stringFlag
	schemaFlag.value = cfg.PGSchema
	flag.Var(&schemaFlag, "pg-schema", "PostgreSQL schema")

	var driverFlag stringFlag
	driverFlag.value = cfg.PGDriver
	flag.Var(&driverFlag, "pg-driver", "PostgreSQL driver name")

	var inboxFlag stringFlag
	inboxFlag.value = cfg.InboxDir
	flag.Var(&inboxFlag, "inbox", "inbox directory")

	var processingFlag stringFlag
	processingFlag.value = cfg.Processing
	flag.Var(&processingFlag, "processing", "processing directory")

	var processedFlag stringFlag
	processedFlag.value = cfg.Processed
	flag.Var(&processedFlag, "processed", "processed directory")

	var failedFlag stringFlag
	failedFlag.value = cfg.Failed
	flag.Var(&failedFlag, "failed", "failed directory")

	var maxJSONFlag int64Flag
	maxJSONFlag.value = cfg.MaxJSONBytes
	flag.Var(&maxJSONFlag, "max-json-bytes", "max analysis.json size in bytes")

	var dryRunFlag boolFlag
	dryRunFlag.value = cfg.DryRun
	flag.Var(&dryRunFlag, "dry-run", "parse only; do not write to DB")

	var onceFlag boolFlag
	onceFlag.value = cfg.Once
	flag.Var(&onceFlag, "once", "process inbox once and exit (default true)")

	var loopFlag boolFlag
	loopFlag.value = cfg.Loop
	flag.Var(&loopFlag, "loop", "loop processing every 5 minutes")

	var loopDelayFlag int64Flag
	loopDelayFlag.value = int64(cfg.LoopDelay / time.Second)
	flag.Var(&loopDelayFlag, "loop-delay-seconds", "loop delay in seconds")

	flag.Parse()

	loadedCfg, err := ingest.LoadConfig(*configPath)
	if err != nil {
		log.Printf("event=config_error error=%s", err)
		os.Exit(1)
	}
	cfg = loadedCfg

	if dsnFlag.set {
		cfg.DSN = dsnFlag.value
	}
	if schemaFlag.set {
		cfg.PGSchema = schemaFlag.value
	}
	if driverFlag.set {
		cfg.PGDriver = driverFlag.value
	}
	if inboxFlag.set {
		cfg.InboxDir = inboxFlag.value
	}
	if processingFlag.set {
		cfg.Processing = processingFlag.value
	}
	if processedFlag.set {
		cfg.Processed = processedFlag.value
	}
	if failedFlag.set {
		cfg.Failed = failedFlag.value
	}
	if maxJSONFlag.set {
		cfg.MaxJSONBytes = maxJSONFlag.value
	}
	if dryRunFlag.set {
		cfg.DryRun = dryRunFlag.value
	}
	if onceFlag.set {
		cfg.Once = onceFlag.value
	}
	if loopFlag.set {
		cfg.Loop = loopFlag.value
	}
	if loopDelayFlag.set {
		cfg.LoopDelay = time.Duration(loopDelayFlag.value) * time.Second
	}

	ctx := context.Background()
	err = ingest.Run(ctx, cfg)
	if err != nil {
		if err == ingest.ExitWithFailures {
			log.Printf("event=batch_completed status=failed")
			os.Exit(2)
		}
		log.Printf("event=ingest_failed error=%s", err)
		os.Exit(1)
	}
	log.Printf("event=ingest_complete status=success")
}
