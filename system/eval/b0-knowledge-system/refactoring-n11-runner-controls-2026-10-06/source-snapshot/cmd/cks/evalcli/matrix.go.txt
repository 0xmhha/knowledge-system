package evalcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/eval"
	sysmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type matrixArm struct {
	ID                string `json:"id"`
	Mode              string `json:"mode"`
	IncludeKnowledge  bool   `json:"include_knowledge"`
	Config            string `json:"config"`
	ConfigSHA256      string `json:"config_sha256"`
	ConfigSHA256After string `json:"config_sha256_after"`
}
type matrixRow struct {
	Sequence int    `json:"sequence"`
	Group    int    `json:"group"`
	Position int    `json:"position"`
	Arm      string `json:"arm"`
	captureRow
}
type matrixReport struct {
	SchemaVersion           int                        `json:"schema_version"`
	State                   string                     `json:"state"`
	StartedAt               time.Time                  `json:"started_at"`
	FinishedAt              *time.Time                 `json:"finished_at,omitempty"`
	RequestSHA256           string                     `json:"request_sha256"`
	BaseConfigSHA256        string                     `json:"base_config_sha256"`
	BinarySHA256            string                     `json:"binary_sha256"`
	BinarySHA256After       string                     `json:"binary_sha256_after,omitempty"`
	WorkingDirectory        string                     `json:"working_directory"`
	Runtime                 map[string]any             `json:"runtime"`
	DatasetIdentity         *setup.DatasetIdentity     `json:"dataset_identity"`
	LockedFilesSHA256       map[string]string          `json:"locked_files_sha256"`
	LockedFilesSHA256After  map[string]string          `json:"locked_files_sha256_after,omitempty"`
	Counts                  map[string]int             `json:"counts"`
	Arms                    []matrixArm                `json:"arms"`
	Rows                    int                        `json:"rows"`
	RowsSHA256              string                     `json:"rows_sha256,omitempty"`
	CallTimeoutNS           int64                      `json:"call_timeout_ns"`
	QualityMetrics          any                        `json:"quality_metrics"`
	Rotation                string                     `json:"rotation"`
	ColdDefinition          string                     `json:"cold_definition"`
	RecordingPolicy         string                     `json:"recording_policy"`
	Errors                  []string                   `json:"errors,omitempty"`
	SourceHEAD              string                     `json:"source_head"`
	SourceHEADAfter         string                     `json:"source_head_after,omitempty"`
	LiveExecutableBits      map[string]bool            `json:"live_executable_bits"`
	LiveExecutableBitsAfter map[string]bool            `json:"live_executable_bits_after,omitempty"`
	EnvironmentBefore       *matrixEnvironment         `json:"environment_before,omitempty"`
	EnvironmentAfter        *matrixEnvironment         `json:"environment_after,omitempty"`
	EnvironmentNote         string                     `json:"environment_note,omitempty"`
	LockedInputCopies       map[string]matrixInputCopy `json:"locked_input_copies,omitempty"`
}

type matrixSession interface {
	Capture(context.Context, eval.CaptureRequest) (eval.RawToolCall, error)
	Close() error
}
type matrixFactory func(context.Context, string, string, time.Duration) (matrixSession, error)

func startMatrixSession(ctx context.Context, binary, cfg string, timeout time.Duration) (matrixSession, error) {
	runner, err := eval.NewRunner(ctx, eval.RunnerOpts{CKSMCPBinary: binary, CKSMCPConfig: cfg, InitializeTimeout: timeout})
	if err != nil {
		return nil, err
	}
	return runner, nil
}

func newMatrixCmd() *cobra.Command {
	var input, cfg, binary, out, binding string
	var warmup, retrieval, warm, cold int
	var timeout time.Duration
	var locks []string
	var environment bool
	var environmentNote string
	cmd := &cobra.Command{Use: "matrix", Short: "Capture eight sequential ablation arms with rotating order and pinned inputs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if environmentNote != "" && !environment {
			return fmt.Errorf("--environment-note requires --environment-ledger")
		}
		reader := matrixInputHashes
		if binding != "" {
			if !environment {
				return fmt.Errorf("--evaluation-binding requires --environment-ledger")
			}
			reader = bindMatrixInputs(binding, reader)
			locks = append(locks, binding)
		}
		if environment {
			return matrixRunWithLedger(ctx, input, cfg, binary, out, warmup, retrieval, warm, cold, timeout, locks, startMatrixSession, reader, observeMatrixEnvironment, environmentNote)
		}
		return matrixRun(ctx, input, cfg, binary, out, warmup, retrieval, warm, cold, timeout, locks, startMatrixSession)
	}}
	cmd.Flags().StringVar(&input, "requests", "", "request-only v2 JSON; include_knowledge is set by the matrix")
	cmd.Flags().StringVar(&cfg, "config", "", "one base MCP configuration for all arms")
	cmd.Flags().StringVar(&binary, "cks-mcp", "", "server binary (default: this executable)")
	cmd.Flags().StringVar(&out, "output", "", "new private output directory; existing paths are refused")
	cmd.Flags().StringVar(&binding, "evaluation-binding", "", "frozen dataset/source/config/binary/model pins; requires environment ledger (not human approval)")
	cmd.Flags().StringSliceVar(&locks, "lock-file", nil, "additional immutable input files to hash before and after")
	cmd.Flags().BoolVar(&environment, "environment-ledger", false, "record host/model before and after; privately copy --lock-file inputs (max 8 MiB each)")
	cmd.Flags().StringVar(&environmentNote, "environment-note", "", "operator's workload note (recorded as an unverified statement)")
	cmd.Flags().IntVar(&warmup, "warmup", 2, "warmup repetitions per request/arm")
	cmd.Flags().IntVar(&retrieval, "retrieval-runs", 5, "retrieval repetitions per request/arm")
	cmd.Flags().IntVar(&warm, "warm-runs", 20, "warm latency repetitions per request/arm")
	cmd.Flags().IntVar(&cold, "cold-runs", 3, "fresh-process repetitions per request/arm; model stays resident")
	cmd.Flags().DurationVar(&timeout, "call-timeout", 90*time.Second, "initialize and per-call deadline")
	return cmd
}

func matrixPlan(raw []byte) (capturePlan, error) {
	p, err := loadCapturePlan(raw)
	if err != nil {
		return p, err
	}
	for _, r := range p.Requests {
		if r.Tool != sysmcp.ToolNameGetForTaskV2 {
			return p, fmt.Errorf("matrix requires v2 requests")
		}
		if _, present := r.Arguments["include_knowledge"]; present {
			return p, fmt.Errorf("matrix owns include_knowledge; omit it from requests")
		}
	}
	return p, nil
}
func armRequest(r eval.CaptureRequest, enabled bool) eval.CaptureRequest {
	a := make(map[string]any, len(r.Arguments)+1)
	for k, v := range r.Arguments {
		a[k] = v
	}
	a["include_knowledge"] = enabled
	r.Arguments = a
	return r
}
func matrixOrder(group int) []int {
	order := make([]int, 8)
	for i := range order {
		order[i] = (group + i) % 8
	}
	return order
}

// Hash the retained archive, live captured repo files, active engine files and
// optional external inputs. Missing live files are captured as "missing": a
// stale/missing-source fixture remains testable, and appearance is detectable.
func matrixInputHashes(cfg *config.Config, baseConfig, input, binary string, extra []string) (map[string]string, *setup.DatasetIdentity, error) {
	graph, err := filepath.Abs(cfg.Backends.CKG.Path)
	if err != nil {
		return nil, nil, err
	}
	version := filepath.Dir(filepath.Dir(graph))
	vector, err := filepath.Abs(cfg.Backends.CKV.Path)
	if err != nil {
		return nil, nil, err
	}
	if graph != filepath.Join(version, "graph", "graph.db") || vector != filepath.Join(version, "vector") {
		return nil, nil, fmt.Errorf("matrix requires a native graph/vector version layout")
	}
	identity, err := setup.InspectVersionIdentity(version)
	if err != nil || identity == nil {
		return nil, nil, fmt.Errorf("matrix requires verified pinned v2 identity: %v", err)
	}
	if err := setup.VerifyRetainedSource(version, identity.Source); err != nil {
		return nil, nil, err
	}
	paths := []string{baseConfig, input, binary, graph, filepath.Join(vector, "manifest.json"), filepath.Join(version, "dataset-identity.json")}
	for _, p := range []string{cfg.Semantic.StorePath, cfg.Vocab.GlossaryPath, cfg.Sanitize.RulesPath} {
		if p != "" {
			paths = append(paths, p)
		}
	}
	dbs, err := filepath.Glob(filepath.Join(vector, "*.db"))
	if err != nil {
		return nil, nil, err
	}
	paths = append(paths, dbs...)
	paths = append(paths, extra...)
	err = filepath.WalkDir(filepath.Join(version, "sources"), func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(filepath.Join(version, "sources", "manifest.json"))
	if err != nil {
		return nil, nil, err
	}
	var captured setup.CapturedSource
	if err := json.Unmarshal(raw, &captured); err != nil {
		return nil, nil, err
	}
	values := make(map[string]string)
	for _, f := range captured.Files {
		if f.OriginID == "repo" && f.Kind == "regular" {
			p := filepath.Join(cfg.Backends.CKG.SourceRoot, filepath.FromSlash(f.Path))
			absolute, err := filepath.Abs(p)
			if err != nil {
				return nil, nil, err
			}
			h, err := shaFile(absolute)
			if os.IsNotExist(err) {
				values[absolute] = "missing"
			} else if err != nil {
				return nil, nil, err
			} else {
				values[absolute] = h
			}
		}
	}
	for _, p := range paths {
		absolute, err := filepath.Abs(p)
		if err != nil {
			return nil, nil, err
		}
		h, err := shaFile(absolute)
		if err != nil {
			return nil, nil, fmt.Errorf("lock %s: %w", absolute, err)
		}
		values[absolute] = h
	}
	return values, identity, nil
}

type matrixInputReader func(*config.Config, string, string, string, []string) (map[string]string, *setup.DatasetIdentity, error)

func matrixRun(ctx context.Context, input, baseConfig, binary, out string, warmup, retrieval, warm, cold int, timeout time.Duration, extra []string, factory matrixFactory) error {
	return matrixRunWithInputs(ctx, input, baseConfig, binary, out, warmup, retrieval, warm, cold, timeout, extra, factory, matrixInputHashes)
}

func matrixRunWithInputs(ctx context.Context, input, baseConfig, binary, out string, warmup, retrieval, warm, cold int, timeout time.Duration, extra []string, factory matrixFactory, readInputs matrixInputReader) (returnErr error) {
	return matrixRunWithLedger(ctx, input, baseConfig, binary, out, warmup, retrieval, warm, cold, timeout, extra, factory, readInputs, nil, "")
}

func matrixRunWithLedger(ctx context.Context, input, baseConfig, binary, out string, warmup, retrieval, warm, cold int, timeout time.Duration, extra []string, factory matrixFactory, readInputs matrixInputReader, probe matrixEnvironmentProbe, note string) (returnErr error) {
	if input == "" || baseConfig == "" || out == "" {
		return fmt.Errorf("matrix requires --requests, --config and --output")
	}
	if timeout <= 0 || warmup < 0 || retrieval < 1 || warm < 0 || cold < 0 || warmup > 1000 || retrieval > 1000 || warm > 1000 || cold > 1000 {
		return fmt.Errorf("matrix counts must be 0..1000 with retrieval>=1 and positive timeout")
	}
	raw, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	plan, err := matrixPlan(raw)
	if err != nil {
		return err
	}
	baseRaw, err := os.ReadFile(baseConfig)
	if err != nil {
		return err
	}
	cfg, err := config.LoadBytes(baseRaw)
	if err != nil {
		return err
	}
	if binary == "" {
		binary, err = os.Executable()
		if err != nil {
			return err
		}
	}
	input, err = filepath.Abs(input)
	if err != nil {
		return err
	}
	baseConfig, err = filepath.Abs(baseConfig)
	if err != nil {
		return err
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	// Output must not become a new source/archive input during collection.
	for _, protected := range []string{cfg.Backends.CKG.SourceRoot, cfg.Backends.CKV.Path} {
		if protected != "" && matrixWithin(protected, out) {
			return fmt.Errorf("matrix output must be outside source and dataset directories")
		}
	}
	if cfg.Backends.CKG.Path != "" && matrixWithin(filepath.Dir(filepath.Dir(cfg.Backends.CKG.Path)), out) {
		return fmt.Errorf("matrix output must be outside source and dataset directories")
	}
	locked, identity, err := readInputs(cfg, baseConfig, input, binary, extra)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	baseSum := sha256.Sum256(baseRaw)
	if hex.EncodeToString(sum[:]) != locked[input] || hex.EncodeToString(baseSum[:]) != locked[baseConfig] {
		return fmt.Errorf("requests/config changed during preflight")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	report := matrixReport{SchemaVersion: 1, State: "running", StartedAt: time.Now().UTC(), RequestSHA256: hex.EncodeToString(sum[:]), BaseConfigSHA256: locked[baseConfig], BinarySHA256: locked[binary], WorkingDirectory: cwd, Runtime: map[string]any{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "logical_cpus": runtime.NumCPU()}, DatasetIdentity: identity, LockedFilesSHA256: locked, Counts: map[string]int{"warmup": warmup, "retrieval": retrieval, "warm_latency": warm, "cold_process": cold}, CallTimeoutNS: int64(timeout), Rotation: "global group index mod 8; left rotate fixed arm list; every group is one request/phase/iteration across 8 arms", ColdDefinition: "fresh MCP process through first tool response; daemon/model residency unchanged, not model-cold", RecordingPolicy: "full SDK response rows.jsonl; serialize and sync after each response outside call timing; no concurrent calls"}
	report.SourceHEAD, report.LiveExecutableBits = matrixLiveState(cfg, locked)
	// Validate all derived configurations before creating output or any process.
	configs := make([][]byte, 0, 8)
	for _, mode := range []string{"baseline", "concept_text", "relations", "combined"} {
		for _, enabled := range []bool{false, true} {
			suffix := "off"
			if enabled {
				suffix = "on"
			}
			id := mode + "_" + suffix
			clone := *cfg
			clone.Semantic.OntologyMode = mode
			clone.Listen.Transport = "stdio"
			clone.Listen.MCPStdio = true
			clone.Logging.MeasureBackendCalls = true
			clone.Logging.FootprintDir = filepath.Join(out, "footprints", id)
			if err := clone.Validate(); err != nil {
				return fmt.Errorf("matrix arm %s: %w", id, err)
			}
			body, err := yaml.Marshal(clone)
			if err != nil {
				return err
			}
			s := sha256.Sum256(body)
			configs = append(configs, body)
			report.Arms = append(report.Arms, matrixArm{ID: id, Mode: mode, IncludeKnowledge: enabled, Config: filepath.Join(out, id+".yaml"), ConfigSHA256: hex.EncodeToString(s[:])})
		}
	}
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	writeReport := func() error {
		body, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(out, "report.json"), append(body, '\n'), 0600)
	}
	if err := writeReport(); err != nil {
		return err
	}
	defer func() {
		if returnErr != nil {
			if probe != nil && report.EnvironmentBefore != nil && report.EnvironmentAfter == nil {
				after := probe(ctx, cfg, identity)
				report.EnvironmentAfter = &after
			}
			report.State = "partial"
			report.Errors = append(report.Errors, returnErr.Error())
			finished := time.Now().UTC()
			report.FinishedAt = &finished
			_ = writeReport()
		}
	}()
	for i, a := range report.Arms {
		if err := os.WriteFile(a.Config, configs[i], 0600); err != nil {
			return err
		}
	}
	// Preserve the exact original input/config, plus resolved profiles.
	if err := os.WriteFile(filepath.Join(out, "requests.json"), raw, 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "base-config.yaml"), baseRaw, 0600); err != nil {
		return err
	}
	if probe != nil {
		report.EnvironmentNote = note
		report.LockedInputCopies, err = copyMatrixInputs(out, extra, locked)
		if err != nil {
			return err
		}
		before := probe(ctx, cfg, identity)
		report.EnvironmentBefore = &before
		if err := writeReport(); err != nil {
			return err
		}
		if !before.Valid {
			return fmt.Errorf("environment preflight incomplete; inspect preserved environment ledger")
		}
	}
	stream, err := os.OpenFile(filepath.Join(out, "rows.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer stream.Close()
	encoder := json.NewEncoder(stream)
	sessions := make([]matrixSession, 8)
	defer func() {
		for i := len(sessions) - 1; i >= 0; i-- {
			if sessions[i] != nil {
				_ = sessions[i].Close()
			}
		}
	}()
	for i, a := range report.Arms {
		if ctx.Err() != nil {
			break
		}
		s, e := factory(ctx, binary, a.Config, timeout)
		if e != nil {
			report.Errors = append(report.Errors, "start warm "+a.ID+": "+e.Error())
		} else {
			sessions[i] = s
		}
	}
	group := 0
	appendRow := func(row matrixRow) error {
		report.Rows++
		row.Sequence = report.Rows
		if err := encoder.Encode(row); err != nil {
			return err
		}
		return stream.Sync()
	}
	for _, phase := range []string{"warmup", "retrieval", "warm_latency", "cold_process"} {
		if phase == "cold_process" {
			for i, s := range sessions {
				if s != nil {
					if e := s.Close(); e != nil {
						report.Errors = append(report.Errors, "close warm "+report.Arms[i].ID+": "+e.Error())
					}
					sessions[i] = nil
				}
			}
		}
		for _, request := range plan.Requests {
			for iteration := 1; iteration <= report.Counts[phase]; iteration++ {
				if ctx.Err() != nil {
					break
				}
				order := matrixOrder(group)
				for position, index := range order {
					if ctx.Err() != nil {
						break
					}
					arm := report.Arms[index]
					row := matrixRow{Group: group, Position: position + 1, Arm: arm.ID, captureRow: captureRow{RequestID: request.ID, Phase: phase, Iteration: iteration}}
					session := sessions[index]
					var started time.Time
					if phase == "cold_process" {
						started = time.Now()
						session, err = factory(ctx, binary, arm.Config, timeout)
						if err != nil {
							session = nil // an interface holding a typed nil is not nil
							row.Error = "start cold: " + err.Error()
							row.ProcessStartThroughResponseNS = time.Since(started).Nanoseconds()
						}
					}
					if session == nil && row.Error == "" {
						row.Error = "warm session unavailable"
					}
					if session != nil {
						callCtx, cancel := context.WithTimeout(ctx, timeout)
						call, e := session.Capture(callCtx, armRequest(request, arm.IncludeKnowledge))
						cancel()
						row.Call = call
						if !started.IsZero() && call.CompletedAt != nil {
							row.ProcessStartThroughResponseNS = call.CompletedAt.Sub(started).Nanoseconds()
						}
						if e != nil {
							row.Error = e.Error()
						}
						var response struct {
							IsError bool `json:"isError"`
						}
						if e := json.Unmarshal(call.Response, &response); e != nil {
							if row.Error == "" {
								row.Error = "missing or invalid SDK response: " + e.Error()
							}
						} else if response.IsError && row.Error == "" {
							row.Error = "tool returned IsError"
						}
						if phase == "cold_process" {
							if e := session.Close(); e != nil {
								report.Errors = append(report.Errors, "close cold "+arm.ID+": "+e.Error())
							}
						}
					}
					if row.Error != "" {
						report.Errors = append(report.Errors, arm.ID+"/"+request.ID+"/"+phase+": "+row.Error)
					}
					if e := appendRow(row); e != nil {
						return e
					}
				}
				group++
			}
		}
	}
	if ctx.Err() != nil {
		report.Errors = append(report.Errors, ctx.Err().Error())
	}
	for i, s := range sessions {
		if s != nil {
			if e := s.Close(); e != nil {
				report.Errors = append(report.Errors, "close warm "+report.Arms[i].ID+": "+e.Error())
			}
			sessions[i] = nil
		}
	}
	if err := stream.Close(); err != nil {
		return err
	}
	report.RowsSHA256, err = shaFile(filepath.Join(out, "rows.jsonl"))
	if err != nil {
		return err
	}
	report.LockedFilesSHA256After, _, err = readInputs(cfg, baseConfig, input, binary, extra)
	if err != nil {
		report.Errors = append(report.Errors, "post-input audit: "+err.Error())
	} else if !reflect.DeepEqual(locked, report.LockedFilesSHA256After) {
		report.Errors = append(report.Errors, "locked input files changed during matrix capture")
	}
	report.BinarySHA256After = report.LockedFilesSHA256After[binary]
	report.SourceHEADAfter, report.LiveExecutableBitsAfter = matrixLiveState(cfg, report.LockedFilesSHA256After)
	if report.SourceHEAD != report.SourceHEADAfter || !reflect.DeepEqual(report.LiveExecutableBits, report.LiveExecutableBitsAfter) {
		report.Errors = append(report.Errors, "live source HEAD or executable bits changed during matrix capture")
	}
	for i, a := range report.Arms {
		h, e := shaFile(a.Config)
		report.Arms[i].ConfigSHA256After = h
		if e != nil || h != a.ConfigSHA256 {
			report.Errors = append(report.Errors, "derived config changed: "+a.ID)
		}
	}
	if probe != nil {
		after := probe(ctx, cfg, identity)
		report.EnvironmentAfter = &after
		if !after.Valid || !sameMatrixEnvironment(*report.EnvironmentBefore, after) {
			report.Errors = append(report.Errors, "host or model identity changed/unavailable; inspect environment ledger")
		}
		for original, copy := range report.LockedInputCopies {
			h, e := shaFile(filepath.Join(out, copy.Path))
			copy.SHA256After = h
			report.LockedInputCopies[original] = copy
			if e != nil || h != copy.SHA256 {
				report.Errors = append(report.Errors, "locked input copy changed: "+original)
			}
		}
	}
	report.State = "captured"
	if len(report.Errors) > 0 {
		report.State = "partial"
	}
	finished := time.Now().UTC()
	report.FinishedAt = &finished
	if err := writeReport(); err != nil {
		return err
	}
	if report.State == "partial" {
		return fmt.Errorf("matrix recorded errors; inspect preserved report and rows")
	}
	return nil
}

// Source metadata is measured independently: a working-tree fixture may be
// intentionally stale relative to the retained identity. Preserve that state.
func matrixLiveState(cfg *config.Config, locked map[string]string) (string, map[string]bool) {
	head := "unavailable"
	if cfg.Backends.CKG.SourceRoot != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		raw, err := exec.CommandContext(ctx, "git", "-C", cfg.Backends.CKG.SourceRoot, "rev-parse", "--verify", "HEAD").Output()
		cancel()
		if err == nil {
			head = strings.TrimSpace(string(raw))
		}
	}
	bits := map[string]bool{}
	root, err := filepath.Abs(cfg.Backends.CKG.SourceRoot)
	if err == nil && cfg.Backends.CKG.SourceRoot != "" {
		for path, hash := range locked {
			if hash != "missing" && strings.HasPrefix(path, root+string(os.PathSeparator)) {
				if info, err := os.Stat(path); err == nil {
					bits[path] = info.Mode()&0111 != 0
				}
			}
		}
	}
	return head, bits
}

func matrixWithin(root, path string) bool {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	if resolved, e := filepath.EvalSymlinks(absolute); e == nil {
		absolute = resolved
	}
	if parent, e := filepath.EvalSymlinks(filepath.Dir(path)); e == nil {
		path = filepath.Join(parent, filepath.Base(path))
	}
	rel, err := filepath.Rel(absolute, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
