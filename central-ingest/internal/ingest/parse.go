package ingest

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
)

var filenamePattern = regexp.MustCompile(`^(.+?)_(.+?)_(\d{8})\.tar\.gz$`)

func ParseFilename(name string) (siteID, deviceID, ymd string, ok bool) {
	base := filepath.Base(name)
	matches := filenamePattern.FindStringSubmatch(base)
	if len(matches) != 4 {
		return "", "", "", false
	}
	return matches[1], matches[2], matches[3], true
}

func ExtractAnalysisJSON(reader io.Reader, maxBytes int64) ([]byte, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("analysis.json not found in archive")
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if filepath.Base(hdr.Name) != "analysis.json" {
			continue
		}
		if hdr.Size > maxBytes {
			return nil, fmt.Errorf("analysis.json exceeds max size")
		}
		limit := io.LimitReader(tarReader, maxBytes+1)
		data, err := io.ReadAll(limit)
		if err != nil {
			return nil, fmt.Errorf("read analysis.json: %w", err)
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("analysis.json exceeds max size")
		}
		return data, nil
	}
}

func ParseAnalysis(data []byte) (Analysis, error) {
	var analysis Analysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return Analysis{}, fmt.Errorf("parse analysis.json: %w", err)
	}
	return analysis, nil
}

func MetricValue(val interface{}) (num *float64, text *string) {
	switch typed := val.(type) {
	case float64:
		return &typed, nil
	case bool:
		str := fmt.Sprintf("%t", typed)
		return nil, &str
	case string:
		return nil, &typed
	case nil:
		return nil, nil
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			str := fmt.Sprintf("%v", typed)
			return nil, &str
		}
		str := string(encoded)
		return nil, &str
	}
}

func IssueCount(val interface{}) (int64, bool) {
	switch typed := val.(type) {
	case float64:
		return int64(typed), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case string:
		return 0, false
	default:
		return 0, false
	}
}
