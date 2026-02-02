package ingest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestParseFilename(t *testing.T) {
	site, device, ymd, ok := ParseFilename("siteA_deviceB_20240201.tar.gz")
	if !ok {
		t.Fatalf("expected match")
	}
	if site != "siteA" || device != "deviceB" || ymd != "20240201" {
		t.Fatalf("unexpected parse results: %s %s %s", site, device, ymd)
	}

	_, _, _, ok = ParseFilename("nope.tar.gz")
	if ok {
		t.Fatalf("expected no match")
	}
}

func TestExtractAnalysisJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gzipWriter)

	content := []byte(`{"schema_version":"v1"}`)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "nested/path/analysis.json", Size: int64(len(content))}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	data, err := ExtractAnalysisJSON(bytes.NewReader(buf.Bytes()), int64(len(content)+10))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestMetricValue(t *testing.T) {
	num, text := MetricValue(float64(1.5))
	if num == nil || *num != 1.5 || text != nil {
		t.Fatalf("expected numeric metric")
	}

	_, text = MetricValue("abc")
	if text == nil || *text != "abc" {
		t.Fatalf("expected text metric")
	}

	_, text = MetricValue(true)
	if text == nil || *text != "true" {
		t.Fatalf("expected bool conversion")
	}

	_, text = MetricValue(map[string]interface{}{"k": "v"})
	if text == nil || *text == "" {
		t.Fatalf("expected json text")
	}
}
