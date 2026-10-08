// b1-evidence-audit replays the public v2 verifier against retained SDK rows.
// It does not contact a model, open a dataset or decide policy/answer quality.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type record struct {
	Sequence      int    `json:"sequence"`
	Arm           string `json:"arm"`
	RequestID     string `json:"request_id"`
	Phase         string `json:"phase"`
	Iteration     int    `json:"iteration"`
	MeasurementID string `json:"measurement_id"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}
type report struct {
	SchemaVersion  int      `json:"schema_version"`
	Scope          string   `json:"scope"`
	InputSHA256    string   `json:"input_sha256"`
	BinarySHA256   string   `json:"binary_sha256"`
	GoVersion      string   `json:"go_version"`
	Records        []record `json:"records"`
	Failed         int      `json:"failed"`
	QualityMetrics any      `json:"quality_metrics"`
}

func digest(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func audit(raw []byte) (report, error) {
	result := report{SchemaVersion: 1, Scope: "public Go EvidencePackV2.Verify replay; archive bytes and human facts are separate", InputSHA256: digest(raw), GoVersion: runtime.Version(), Records: []record{}}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 8<<20)
	for scanner.Scan() {
		var row struct {
			Sequence  int    `json:"sequence"`
			Arm       string `json:"arm"`
			RequestID string `json:"request_id"`
			Phase     string `json:"phase"`
			Iteration int    `json:"iteration"`
			Error     string `json:"error"`
			Call      struct {
				MeasurementID  string `json:"measurement_id"`
				TransportError string `json:"transport_error"`
				Response       struct {
					IsError           bool            `json:"isError"`
					StructuredContent json.RawMessage `json:"structuredContent"`
				} `json:"response"`
			} `json:"call"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return result, fmt.Errorf("row JSON decode: %w", err)
		}
		r := record{Sequence: row.Sequence, Arm: row.Arm, RequestID: row.RequestID, Phase: row.Phase, Iteration: row.Iteration, MeasurementID: row.Call.MeasurementID, Status: "valid_contract"}
		switch {
		case row.Error != "" || row.Call.TransportError != "" || row.Call.Response.IsError:
			r.Status, r.Error = "capture_error", "SDK/initialize/transport failure; inspect original capture"
		case len(row.Call.Response.StructuredContent) == 0 || bytes.Equal(row.Call.Response.StructuredContent, []byte("null")):
			r.Status, r.Error = "invalid_contract", "missing structured v2 evidence"
		default:
			var pack contract.EvidencePackV2
			err := json.Unmarshal(row.Call.Response.StructuredContent, &pack)
			if err == nil {
				err = evidencev2.Verify(pack)
			}
			if err != nil {
				r.Status, r.Error = "invalid_contract", err.Error()
			}
		}
		if r.Status != "valid_contract" {
			result.Failed++
		}
		result.Records = append(result.Records, r)
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func run(input, output string) (int, error) {
	if input == "" || output == "" {
		return 1, fmt.Errorf("--input and --output are required")
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		return 1, err
	}
	result, err := audit(raw)
	if err != nil {
		return 1, err
	}
	executable, err := os.Executable()
	if err != nil {
		return 1, err
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		return 1, err
	}
	result.BinarySHA256 = digest(binary)
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return 1, err
	}
	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 1, err
	}
	_, writeErr := f.Write(append(encoded, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return 1, writeErr
	}
	if closeErr != nil {
		return 1, closeErr
	}
	if result.Failed > 0 {
		return 2, nil
	}
	return 0, nil
}

func main() {
	input := flag.String("input", "", "retained matrix rows.jsonl")
	output := flag.String("output", "", "new contract-audit JSON; never overwritten")
	flag.Parse()
	code, err := run(*input, *output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "b1-evidence-audit:", err)
	}
	os.Exit(code)
}
