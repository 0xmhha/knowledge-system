package evalcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCapturePlanRejectsExtraGoldDuplicateAndTrailingPayload(t *testing.T) {
	valid := `{"schema_version":1,"requests":[{"id":"v2","tool":"cks.context.get_for_task_v2","arguments":{"prompt":"Alpha","include_knowledge":true}}]}`
	if _, err := loadCapturePlan([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{"schema_version":1,"gold":"secret","requests":[]}`,
		`{"schema_version":1,"requests":[{"id":"x","tool":"cks.context.get_for_task","arguments":{"prompt":"Alpha"},"expected_answer":"gold"}]}`,
		`{"schema_version":1,"requests":[{"id":"x","tool":"cks.context.get_for_task","arguments":{"prompt":"Alpha"}},{"id":"x","tool":"cks.context.get_for_task","arguments":{"prompt":"Beta"}}]}`,
		valid + `{}`,
	} {
		if _, err := loadCapturePlan([]byte(raw)); err == nil {
			t.Fatalf("accepted malformed/gold input: %s", raw)
		}
	}
}

func TestCaptureInitializeDeadlineKeepsFailedReport(t *testing.T) {
	root := t.TempDir()
	input, config, binary, output := filepath.Join(root, "requests.json"), filepath.Join(root, "config"), filepath.Join(root, "stalled-mcp"), filepath.Join(root, "report.json")
	if err := os.WriteFile(input, []byte(`{"schema_version":1,"requests":[{"id":"v2","tool":"cks.context.get_for_task_v2","arguments":{"prompt":"Alpha"}}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexec sleep 10\n"), 0700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err := captureRun(context.Background(), input, config, binary, output, 0, 1, 0, 0, 30*time.Millisecond)
	if err == nil {
		t.Fatal("stalled initialization accepted")
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("initialize deadline did not bound process cleanup")
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var report captureReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.State != "partial" || len(report.Errors) == 0 || !strings.Contains(strings.Join(report.Errors, " "), "deadline") {
		t.Fatalf("deadline evidence lost: %s", raw)
	}
	if report.CallTimeoutNS != int64(30*time.Millisecond) {
		t.Fatal("deadline was not recorded")
	}
}

func TestCaptureRefusesExistingOutputBeforeServerStartup(t *testing.T) {
	root := t.TempDir()
	input, config, binary, output := filepath.Join(root, "requests.json"), filepath.Join(root, "config"), filepath.Join(root, "not-executable"), filepath.Join(root, "existing.json")
	for path, body := range map[string]string{
		input:  `{"schema_version":1,"requests":[{"id":"v1","tool":"cks.context.get_for_task","arguments":{"prompt":"Alpha"}}]}`,
		config: "fixture", binary: "fixture", output: "preserve me",
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := captureRun(context.Background(), input, config, binary, output, 0, 1, 0, 0, time.Second)
	if !os.IsExist(err) {
		t.Fatalf("should refuse output before starting invalid binary: %v", err)
	}
	raw, _ := os.ReadFile(output)
	if string(raw) != "preserve me" {
		t.Fatal("output overwritten")
	}

	newOutput := filepath.Join(root, "failed.json")
	err = captureRun(context.Background(), input, config, binary, newOutput, 0, 1, 0, 1, time.Second)
	if err == nil {
		t.Fatal("invalid MCP binary was accepted")
	}
	raw, readErr := os.ReadFile(newOutput)
	if readErr != nil || len(raw) == 0 {
		t.Fatal("startup errors did not leave a report", readErr)
	}
	if _, err := loadCapturePlan(raw); err == nil {
		t.Fatal("output unexpectedly looked like input")
	}
}
