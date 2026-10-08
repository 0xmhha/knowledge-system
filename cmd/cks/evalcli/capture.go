package evalcli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/eval"
	"github.com/spf13/cobra"
)

type capturePlan struct {
	SchemaVersion int                   `json:"schema_version"`
	Requests      []eval.CaptureRequest `json:"requests"`
}

type captureRow struct {
	RequestID                     string           `json:"request_id"`
	Phase                         string           `json:"phase"`
	Iteration                     int              `json:"iteration"`
	Call                          eval.RawToolCall `json:"call"`
	ProcessStartThroughResponseNS int64            `json:"process_start_through_response_ns,omitempty"`
	Error                         string           `json:"error,omitempty"`
}

type captureReport struct {
	SchemaVersion     int            `json:"schema_version"`
	StartedAt         time.Time      `json:"started_at"`
	FinishedAt        time.Time      `json:"finished_at"`
	RequestSHA256     string         `json:"request_sha256"`
	ConfigSHA256      string         `json:"config_sha256"`
	BinarySHA256      string         `json:"binary_sha256"`
	ConfigSHA256After string         `json:"config_sha256_after"`
	BinarySHA256After string         `json:"binary_sha256_after"`
	QualityMetrics    any            `json:"quality_metrics"`
	State             string         `json:"state"`
	ColdDefinition    string         `json:"cold_definition"`
	Counts            map[string]int `json:"counts"`
	CallTimeoutNS     int64          `json:"call_timeout_ns"`
	Rows              []captureRow   `json:"rows"`
	Errors            []string       `json:"errors,omitempty"`
}

func loadCapturePlan(raw []byte) (capturePlan, error) {
	var plan capturePlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return plan, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return plan, fmt.Errorf("capture input must be one JSON object")
	}
	if plan.SchemaVersion != 1 || len(plan.Requests) == 0 {
		return plan, fmt.Errorf("capture requires schema_version=1 and requests")
	}
	ids := map[string]bool{}
	for _, request := range plan.Requests {
		if err := request.Validate(); err != nil {
			return plan, err
		}
		if ids[request.ID] {
			return plan, fmt.Errorf("duplicate capture request ID")
		}
		ids[request.ID] = true
	}
	return plan, nil
}

func shaFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func newCaptureCmd() *cobra.Command {
	var input, config, binary, output string
	var warmup, retrieval, warm, cold int
	var timeout time.Duration
	cmd := &cobra.Command{Use: "capture", Short: "Record v1/v2 MCP responses and separate timing phases without scoring", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return captureRun(ctx, input, config, binary, output, warmup, retrieval, warm, cold, timeout)
		}}
	cmd.Flags().StringVar(&input, "requests", "", "request-only JSON manifest (required; no gold answers)")
	cmd.Flags().StringVar(&config, "config", "", "MCP configuration file (required)")
	cmd.Flags().StringVar(&binary, "cks-mcp", "", "MCP server binary (default: this executable)")
	cmd.Flags().StringVar(&output, "output", "", "new output JSON file (required; existing files are refused)")
	cmd.Flags().IntVar(&warmup, "warmup", 2, "warmup calls per request, recorded but excluded from measured phases")
	cmd.Flags().IntVar(&retrieval, "retrieval-runs", 5, "retrieval repetitions per request in one process")
	cmd.Flags().IntVar(&warm, "warm-runs", 20, "warm latency repetitions per request after warmup")
	cmd.Flags().IntVar(&cold, "cold-runs", 3, "fresh MCP process repetitions per request; model residency is unchanged")
	cmd.Flags().DurationVar(&timeout, "call-timeout", 90*time.Second, "per-call and MCP initialize deadline")
	return cmd
}

func captureRun(ctx context.Context, input, config, binary, output string, warmup, retrieval, warm, cold int, timeout time.Duration) error {
	if input == "" || config == "" || output == "" {
		return fmt.Errorf("capture requires --requests, --config and --output")
	}
	if timeout <= 0 {
		return fmt.Errorf("capture call-timeout must be positive")
	}
	if warmup < 0 || warm < 0 || cold < 0 || retrieval < 1 || warmup > 1000 || warm > 1000 || cold > 1000 || retrieval > 1000 {
		return fmt.Errorf("capture counts must be 0..1000 with retrieval-runs >= 1")
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	plan, err := loadCapturePlan(raw)
	if err != nil {
		return err
	}
	if binary == "" {
		binary, err = os.Executable()
		if err != nil {
			return err
		}
	}
	configSHA, err := shaFile(config)
	if err != nil {
		return err
	}
	binarySHA, err := shaFile(binary)
	if err != nil {
		return err
	}
	requestSum := sha256.Sum256(raw)
	report := captureReport{SchemaVersion: 1, StartedAt: time.Now().UTC(), RequestSHA256: hex.EncodeToString(requestSum[:]),
		ConfigSHA256: configSHA, BinarySHA256: binarySHA, State: "captured",
		ColdDefinition: "fresh MCP process through first tool response; daemon/model residency unchanged, not model-cold",
		Counts:         map[string]int{"warmup": warmup, "retrieval": retrieval, "warm_latency": warm, "cold_process": cold}, Rows: []captureRow{}}
	report.CallTimeoutNS = timeout.Nanoseconds()
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	create := func() (*eval.Runner, error) {
		return eval.NewRunner(ctx, eval.RunnerOpts{CKSMCPBinary: binary, CKSMCPConfig: config, InitializeTimeout: timeout})
	}
	appendCall := func(runner *eval.Runner, request eval.CaptureRequest, phase string, iteration int, started time.Time) {
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		call, callErr := runner.Capture(callCtx, request)
		cancel()
		row := captureRow{RequestID: request.ID, Phase: phase, Iteration: iteration, Call: call}
		if !started.IsZero() && call.CompletedAt != nil {
			row.ProcessStartThroughResponseNS = call.CompletedAt.Sub(started).Nanoseconds()
		}
		if callErr != nil {
			row.Error = callErr.Error()
			report.Errors = append(report.Errors, request.ID+": "+row.Error)
		}
		var response struct {
			IsError bool `json:"isError"`
		}
		_ = json.Unmarshal(call.Response, &response)
		if response.IsError {
			report.Errors = append(report.Errors, request.ID+": tool returned IsError")
		}
		report.Rows = append(report.Rows, row)
	}
	runner, startErr := create()
	if startErr != nil {
		report.Errors = append(report.Errors, "start warm session: "+startErr.Error())
	} else {
		for _, request := range plan.Requests {
			for _, phase := range []string{"warmup", "retrieval", "warm_latency"} {
				for iteration := 1; iteration <= report.Counts[phase]; iteration++ {
					if ctx.Err() != nil {
						break
					}
					appendCall(runner, request, phase, iteration, time.Time{})
				}
			}
		}
		if err := runner.Close(); err != nil {
			report.Errors = append(report.Errors, "close warm session: "+err.Error())
		}
	}
	for _, request := range plan.Requests {
		for iteration := 1; iteration <= cold; iteration++ {
			if ctx.Err() != nil {
				break
			}
			started := time.Now()
			runner, err := create()
			if err != nil {
				report.Rows = append(report.Rows, captureRow{RequestID: request.ID, Phase: "cold_process", Iteration: iteration,
					Error: err.Error(), ProcessStartThroughResponseNS: time.Since(started).Nanoseconds()})
				report.Errors = append(report.Errors, "start cold session: "+err.Error())
				continue
			}
			appendCall(runner, request, "cold_process", iteration, started)
			if err := runner.Close(); err != nil {
				report.Errors = append(report.Errors, "close cold session: "+err.Error())
			}
		}
	}
	if ctx.Err() != nil {
		report.Errors = append(report.Errors, ctx.Err().Error())
	}
	report.ConfigSHA256After, err = shaFile(config)
	if err != nil || report.ConfigSHA256After != report.ConfigSHA256 {
		report.Errors = append(report.Errors, "config changed or disappeared during capture")
	}
	report.BinarySHA256After, err = shaFile(binary)
	if err != nil || report.BinarySHA256After != report.BinarySHA256 {
		report.Errors = append(report.Errors, "binary changed or disappeared during capture")
	}
	if len(report.Errors) > 0 {
		report.State = "partial"
	}
	report.FinishedAt = time.Now().UTC()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if report.State == "partial" {
		return fmt.Errorf("capture recorded errors; inspect the preserved report")
	}
	return nil
}
