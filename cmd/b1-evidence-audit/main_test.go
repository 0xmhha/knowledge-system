package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func fixturePack(t *testing.T) contract.EvidencePackV2 {
	t.Helper()
	c := contract.CitationV2{File: "main.go", StartLine: 3, EndLine: 3, CommitHash: strings.Repeat("c", 40), ProjectID: "fixture", DatasetID: strings.Repeat("d", 64), SnapshotID: strings.Repeat("b", 64), SourceMode: "committed", BaseCommit: strings.Repeat("c", 40), OriginID: "repo", FileSHA256: strings.Repeat("a", 64), ContentSHA256: strings.Repeat("a", 64)}
	p := contract.EvidencePackV2{FormatVersion: 2, Coordinates: contract.V2Coordinates{ProjectID: c.ProjectID, DatasetID: c.DatasetID, SnapshotID: c.SnapshotID, SourceMode: c.SourceMode, BaseCommit: c.BaseCommit}, Query: "Alpha", Citations: []contract.CitationV2{c}, Bodies: []contract.BodyV2{{Citation: c, Text: "fixture"}}, EvidenceState: "complete", Metadata: contract.V2Metadata{IntegrityHashAlgo: "sha256-v2"}}
	if err := evidencev2.Stamp(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRetainedRowsKeepContractAndCaptureFailures(t *testing.T) {
	pack := fixturePack(t)
	valid := map[string]any{"sequence": 1, "arm": "baseline_on", "request_id": "alpha", "phase": "retrieval", "iteration": 1, "call": map[string]any{"measurement_id": "query-1", "response": map[string]any{"structuredContent": pack}}}
	raw, _ := json.Marshal(valid)
	pack.Metadata.IntegrityHash = strings.Repeat("0", 64)
	invalid, _ := json.Marshal(map[string]any{"sequence": 2, "call": map[string]any{"measurement_id": "query-2", "response": map[string]any{"structuredContent": pack}}})
	data := append(append(raw, '\n'), append(invalid, '\n')...)
	data = append(data, []byte("{\"sequence\":3,\"error\":\"private fake diagnostic\",\"call\":{}}\n{\"sequence\":4,\"call\":{}}\n")...)
	r, err := audit(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Records) != 4 || r.Failed != 3 || r.InputSHA256 != digest(data) || r.QualityMetrics != nil {
		t.Fatalf("incorrect audit summary: %+v", r)
	}
	if r.Records[0].Status != "valid_contract" || r.Records[0].MeasurementID != "query-1" || r.Records[0].RequestID != "alpha" {
		t.Fatal(r.Records[0])
	}
	if r.Records[1].Status != "invalid_contract" || r.Records[2].Status != "capture_error" || r.Records[3].Status != "invalid_contract" {
		t.Fatal(r.Records)
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), "private fake") {
		t.Fatal("copied raw capture failure into audit")
	}
}

func TestMalformedRowsAreNotPublishedAsValidContract(t *testing.T) {
	if _, err := audit([]byte("{\"call\":{}\n")); err == nil {
		t.Fatal("accepted malformed NDJSON")
	}
}
