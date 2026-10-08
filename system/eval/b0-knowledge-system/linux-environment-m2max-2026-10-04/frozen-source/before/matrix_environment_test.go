package evalcli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/eval"
)

// Opt-in integration probe for the actual Linux runtime; ordinary unit tests
// never write this artifact or assert a developer host's resource state.
func TestMatrixLinuxEnvironmentProbe(t *testing.T) {
	out := os.Getenv("KS_MATRIX_ENV_PROBE_OUT")
	if runtime.GOOS != "linux" || out == "" {
		t.Skip("actual Linux artifact probe not requested")
	}
	cfg := config.Default()
	cfg.Backends.CKV.Provider = "mock"
	cfg.Backends.CKV.EmbedModel = "mock-feature-hash-v1"
	identity := &setup.DatasetIdentity{EmbeddingIdentity: json.RawMessage(`{"dim":64,"model":"mock-feature-hash-v1","checksum":"provider=mock;model=mock-feature-hash-v1;dim=64;pooling=;normalize=l2"}`)}
	observed := observeMatrixEnvironment(context.Background(), cfg, identity)
	raw, err := json.MarshalIndent(observed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if !observed.Valid {
		t.Fatalf("actual Linux environment incomplete: %v", observed.Errors)
	}
}

func modelTestPin() *setup.DatasetIdentity {
	return &setup.DatasetIdentity{EmbeddingIdentity: json.RawMessage(`{"Provider":"ollama","Model":"bge-m3:latest","Dim":3,"model_digest":"` + strings.Repeat("a", 64) + `","runtime_context_tokens":8192,"runtime_batch_tokens":8192}`)}
}

func TestMatrixModelIdentityProbe(t *testing.T) {
	for _, scenario := range []string{"stable", "wrong_digest", "changed_digest", "wrong_dimension", "redirect", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			tags, embeds := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/version":
					if scenario == "redirect" {
						http.Redirect(w, r, "http://example.invalid", http.StatusFound)
						return
					}
					_, _ = w.Write([]byte(`{"version":"test-version"}`))
				case "/api/tags":
					tags++
					digest := strings.Repeat("a", 64)
					if scenario == "wrong_digest" || scenario == "changed_digest" && tags > 1 {
						digest = strings.Repeat("b", 64)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]string{"name": "bge-m3:latest", "digest": digest}}})
				case "/api/embed":
					embeds++
					var body map[string]any
					if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&body) != nil {
						t.Error("invalid embed request")
					}
					if body["truncate"] != false || body["model"] != "bge-m3:latest" || body["input"] != "CKS eval matrix environment identity probe" {
						t.Error(body)
					}
					options, _ := body["options"].(map[string]any)
					if options["num_ctx"] != float64(8192) || options["num_batch"] != float64(8192) {
						t.Error(options)
					}
					vector := []float64{0.2, 0.3, 0.4}
					if scenario == "wrong_dimension" {
						vector = vector[:2]
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float64{vector}})
				case "/api/ps":
					_, _ = w.Write([]byte(`{"models":[{"name":"bge-m3:latest","digest":"digest","size":123,"size_vram":100,"private_field":"not-recorded"},{"name":"other-private-model"}]}`))
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			cfg := config.Default()
			cfg.Backends.CKV.Provider = "ollama"
			cfg.Backends.CKV.EmbedModel = "bge-m3"
			cfg.Backends.CKV.OllamaURL = server.URL
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			result := observeMatrixModel(ctx, cfg, modelTestPin())
			if result.Valid != (scenario == "stable") {
				t.Fatalf("unexpected observation: %+v", result)
			}
			if scenario == "stable" && (tags != 2 || embeds != 1 || result.Dimension != 3 || result.ServerVersion != "test-version" || strings.Contains(string(result.Residency), "not-recorded") || strings.Contains(string(result.Residency), "other-private-model")) {
				t.Fatal(result, tags, embeds)
			}
			if (scenario == "wrong_digest" || scenario == "redirect" || scenario == "cancelled") && embeds != 0 {
				t.Fatal("embedded after failed identity preflight")
			}
		})
	}
}

func TestMatrixModelProbeMockAndEndpointContract(t *testing.T) {
	cfg := config.Default()
	cfg.Backends.CKV.Provider = "mock"
	cfg.Backends.CKV.EmbedModel = ""
	identity := &setup.DatasetIdentity{EmbeddingIdentity: json.RawMessage(`{"Provider":"mock","Model":"mock-feature-hash-v1","Dim":16}`)}
	if got := observeMatrixModel(context.Background(), cfg, identity); !got.Valid || got.Dimension != 16 || got.Endpoint != "" {
		t.Fatal(got)
	}
	identity.EmbeddingIdentity = json.RawMessage(`{"dim":64,"model":"mock-feature-hash-v1","checksum":"provider=mock;model=mock-feature-hash-v1;dim=64;pooling=;normalize=l2"}`)
	if got := observeMatrixModel(context.Background(), cfg, identity); !got.Valid || got.Dimension != 64 {
		t.Fatal(got)
	}
	identity.EmbeddingIdentity = json.RawMessage(`{"dim":64,"model":"mock-feature-hash-v1","checksum":"provider=mock;model=mock-feature-hash-v1;dim=16;pooling=;normalize=l2"}`)
	if got := observeMatrixModel(context.Background(), cfg, identity); got.Valid {
		t.Fatal("accepted inconsistent legacy checksum", got)
	}
	cfg.Backends.CKV.Provider = "ollama"
	cfg.Backends.CKV.EmbedModel = "bge-m3"
	for _, endpoint := range []string{"https://127.0.0.1", "http://example.invalid", "http://user:pass@127.0.0.1", "http://127.0.0.1/path", "http://127.0.0.1?secret=value"} {
		cfg.Backends.CKV.OllamaURL = endpoint
		got := observeMatrixModel(context.Background(), cfg, modelTestPin())
		if got.Valid || len(got.Errors) != 1 || got.Errors[0] != "model_endpoint_must_be_loopback_http" {
			t.Fatal(got)
		}
	}
}

func TestMatrixProcessPressureParsing(t *testing.T) {
	top, count := parseMatrixProcesses("10 1 12.4 100\n20 1 50.0 200\n30 1 NaN 5\ninvalid\n")
	if count != 2 || len(top) != 2 || top[0].PID != 20 || top[0].RSSBytes != 204800 || top[1].CPUPercent != 12.4 {
		t.Fatal(top, count)
	}
}

func TestMatrixEnvironmentLedgerLifecycle(t *testing.T) {
	for _, scenario := range []string{"stable", "preflight_failure", "model_change", "copy_change"} {
		t.Run(scenario, func(t *testing.T) {
			input, cfg, binary, out := matrixTestFiles(t)
			gold := filepath.Join(filepath.Dir(input), "gold.json")
			goldBytes := []byte(`{"candidate_answer":"never_to_backend"}`)
			if err := os.WriteFile(gold, goldBytes, 0600); err != nil {
				t.Fatal(err)
			}
			calls, starts, probes := 0, 0, 0
			var sessions []*matrixFake
			factory := func(context.Context, string, string, time.Duration) (matrixSession, error) {
				starts++
				s := &matrixFake{capture: func(r eval.CaptureRequest) error {
					calls++
					body, _ := json.Marshal(r.Arguments)
					if strings.Contains(string(body), "never_to_backend") {
						t.Fatal("gold entered search request")
					}
					if scenario == "copy_change" {
						if err := os.WriteFile(filepath.Join(out, "locked-inputs", "000-gold.json"), []byte("mutated"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					return nil
				}}
				sessions = append(sessions, s)
				return s, nil
			}
			probe := func(context.Context, *config.Config, *setup.DatasetIdentity) matrixEnvironment {
				probes++
				e := matrixEnvironment{At: time.Now().UTC(), Valid: true, Model: matrixModelObservation{Provider: "mock", Model: "mock", Dimension: 16, Valid: true}, Hardware: matrixHardware{MemoryBytes: 64, CPUBrand: "test", LogicalCPUs: 2, Load: "variable"}}
				if probes == 1 {
					if starts != 0 {
						t.Fatal("started session before environment preflight")
					}
					copy, err := os.ReadFile(filepath.Join(out, "locked-inputs", "000-gold.json"))
					if err != nil || string(copy) != string(goldBytes) {
						t.Fatal("missing private input copy", err)
					}
					if scenario == "preflight_failure" {
						e.Valid = false
						e.Errors = []string{"model_unavailable"}
					}
				} else if scenario != "preflight_failure" {
					if calls != 16 {
						t.Fatalf("post environment observed before responses: %d", calls)
					}
					for _, s := range sessions {
						if !s.closed {
							t.Fatal("post environment observed with open sessions")
						}
					}
					e.Hardware.Load = "normal variable pressure"
					if scenario == "model_change" {
						e.Model.Digest = "changed"
					}
				}
				return e
			}
			err := matrixRunWithLedger(context.Background(), input, cfg, binary, out, 0, 1, 0, 0, time.Second, []string{gold, gold}, factory, matrixTestInputReader, probe, "unverified operator note")
			if (err == nil) != (scenario == "stable") {
				t.Fatalf("unexpected result: %v", err)
			}
			data, e := os.ReadFile(filepath.Join(out, "report.json"))
			if e != nil {
				t.Fatal(e)
			}
			var report matrixReport
			if e = json.Unmarshal(data, &report); e != nil {
				t.Fatal(e)
			}
			if probes != 2 || report.EnvironmentBefore == nil || report.EnvironmentAfter == nil || report.EnvironmentNote != "unverified operator note" || len(report.LockedInputCopies) != 1 || report.QualityMetrics != nil {
				t.Fatal(report, probes)
			}
			if scenario == "preflight_failure" {
				if starts != 0 || report.Rows != 0 || report.State != "partial" {
					t.Fatal(report, starts)
				}
				if _, e := os.Stat(filepath.Join(out, "rows.jsonl")); !os.IsNotExist(e) {
					t.Fatal("query rows created after failed preflight")
				}
			} else {
				if report.Rows != 16 || report.LockedInputCopies[gold].SHA256After == "" {
					t.Fatal(report)
				}
				info, e := os.Stat(filepath.Join(out, "locked-inputs", "000-gold.json"))
				if e != nil || info.Mode().Perm() != 0600 {
					t.Fatal("input copy permissions", e)
				}
			}
		})
	}
}
