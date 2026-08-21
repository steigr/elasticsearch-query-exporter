package ecslog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for input, want := range cases {
		got, err := ParseLevel(input)
		if err != nil {
			t.Errorf("ParseLevel(%q): unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseLevel_Invalid(t *testing.T) {
	if _, err := ParseLevel("verbose"); err == nil {
		t.Fatal("expected error for unknown level")
	}
}

func TestNew_RenamesFieldsToECS(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr})
	logger := slog.New(handler).With("ecs.version", ecsVersion)

	logger.Info("hello", "custom", "value")

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}

	for _, key := range []string{"@timestamp", "log.level", "message", "ecs.version", "custom"} {
		if _, ok := out[key]; !ok {
			t.Errorf("expected key %q in log line, got %v", key, out)
		}
	}
	if _, ok := out["time"]; ok {
		t.Error("expected \"time\" key to be renamed away")
	}
	if _, ok := out["level"]; ok {
		t.Error("expected \"level\" key to be renamed away")
	}
	if out["log.level"] != "info" {
		t.Errorf("log.level = %v, want lowercase \"info\"", out["log.level"])
	}
	if out["message"] != "hello" {
		t.Errorf("message = %v, want \"hello\"", out["message"])
	}
}

func TestErr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr}))

	logger.Error("failed", Err(errString("boom")))

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}
	errGroup, ok := out["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected \"error\" group, got %v", out)
	}
	if errGroup["message"] != "boom" {
		t.Errorf("error.message = %v, want \"boom\"", errGroup["message"])
	}
}

type errString string

func (e errString) Error() string { return string(e) }
