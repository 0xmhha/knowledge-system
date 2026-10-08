package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	sysmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// This opt-in SDK diagnostic uses only private copies of already built DEV
// fixtures. Never point it at FINAL inputs. It produces no quality verdict.
func TestRetainedGuardLiveSDK(t *testing.T) {
	casesPath := os.Getenv("B0_RETAINED_GUARD_CASES")
	if casesPath == "" {
		t.Skip("requires explicit DEV fixture descriptors and a built cks binary")
	}
	binary, output := os.Getenv("B0_RETAINED_GUARD_BINARY"), os.Getenv("B0_RETAINED_GUARD_OUTPUT")
	if binary == "" || output == "" {
		t.Fatal("binary and new output file required")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("output must be a new file")
	}
	var cases []struct {
		ID, Version, MixedVersion, Config, MixCode string
		SourceGuards                               bool
	}
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) == 0 {
		t.Fatalf("cases: %v", err)
	}
	var rows []map[string]any
	defer func() {
		raw, err := json.MarshalIndent(map[string]any{"diagnostic_only": true, "quality_metrics": nil, "official_verdict": nil, "cases_sha256": guardHash(raw), "rows": rows, "test_failed": t.Failed()}, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Error(err)
			return
		}
		_, err = f.Write(append(raw, '\n'))
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Errorf("write report: %v %v", err, closeErr)
		}
	}()
	for _, c := range cases {
		t.Run(c.ID, func(t *testing.T) {
			if !strings.Contains(c.ID, "DEV") || c.MixCode != "snapshot_mismatch" {
				t.Fatal("only explicit DEV coordinate guards permitted")
			}
			originalSeal, otherSeal := guardSeal(t, c.Version), guardSeal(t, c.MixedVersion)
			original, err := setup.InspectVersionIdentity(c.Version)
			if err != nil || original == nil {
				t.Fatalf("original: %v", err)
			}
			other, err := setup.InspectVersionIdentity(c.MixedVersion)
			if err != nil || other == nil || original.DatasetID == other.DatasetID {
				t.Fatalf("independent mixed version: %v", err)
			}
			version := filepath.Join(t.TempDir(), "pinned")
			guardCopy(t, c.Version, version)
			copySeal := guardSeal(t, version)
			cfg, err := config.Load(c.Config)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Backends.CKG.Path = filepath.Join(version, "graph", "graph.db")
			cfg.Backends.CKV.Path = filepath.Join(version, "vector")
			cfg.Logging.FootprintDir = filepath.Join(t.TempDir(), "footprints")
			cfg.Logging.AuditDir = ""
			cfg.Semantic.StorePath = ""
			cfgPath := filepath.Join(t.TempDir(), "cks.yaml")
			if err := config.SaveNew(cfgPath, cfg); err != nil {
				t.Fatal(err)
			}
			binaryBefore, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			configBefore, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			runner, err := NewRunner(ctx, RunnerOpts{CKSMCPBinary: binary, CKSMCPConfig: cfgPath, Env: os.Environ(), InitializeTimeout: 30 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer runner.Close()
			capture := func(id, expectedCode string) {
				callCtx, done := context.WithTimeout(ctx, 30*time.Second)
				defer done()
				request := CaptureRequest{ID: id, Tool: sysmcp.ToolNameGetForTaskV2, Arguments: map[string]any{"prompt": "What value does Alpha return?"}}
				row, err := runner.Capture(callCtx, request)
				record := map[string]any{"id": id, "request": request, "raw_call": row, "expected_code": expectedCode, "dataset_id": original.DatasetID, "source_snapshot_id": original.Source.SnapshotID}
				rows = append(rows, record)
				if err != nil {
					t.Errorf("capture %s: %v", id, err)
					return
				}
				var response struct {
					IsError           bool            `json:"isError"`
					StructuredContent json.RawMessage `json:"structuredContent"`
				}
				if err := json.Unmarshal(row.Response, &response); err != nil {
					t.Error(err)
					return
				}
				if expectedCode == "" {
					var pack contract.EvidencePackV2
					if err := json.Unmarshal(response.StructuredContent, &pack); err != nil {
						t.Error(err)
						return
					}
					verifyErr := evidencev2.Verify(pack)
					record["go_verify_valid"] = verifyErr == nil
					if response.IsError || verifyErr != nil || len(pack.Citations) == 0 || pack.Coordinates.DatasetID != original.DatasetID {
						t.Errorf("invalid positive control %s: error=%v verify=%v citations=%d", id, response.IsError, verifyErr, len(pack.Citations))
					}
				} else {
					var structured map[string]string
					if err := json.Unmarshal(response.StructuredContent, &structured); err != nil {
						t.Error(err)
						return
					}
					record["actual_code"] = structured["code"]
					if !response.IsError || structured["code"] != expectedCode {
						t.Errorf("%s: want %s got error=%v %s", id, expectedCode, response.IsError, structured["code"])
					}
					for _, detail := range []string{version, "PRIVATE_TOKEN", "never-return-this"} {
						if strings.Contains(string(row.Response), detail) {
							t.Errorf("public response leaked %s", detail)
						}
					}
				}
			}
			capture(c.ID+"-baseline", "")
			vector := filepath.Join(version, "vector")
			saved := vector + "-saved"
			if err := os.Rename(vector, saved); err != nil {
				t.Fatal(err)
			}
			guardCopy(t, filepath.Join(c.MixedVersion, "vector"), vector)
			capture(c.ID+"-mixed", c.MixCode)
			if err := os.RemoveAll(vector); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(saved, vector); err != nil {
				t.Fatal(err)
			}
			capture(c.ID+"-restored", "")
			if c.SourceGuards {
				blobs := filepath.Join(version, "sources", "blobs")
				files, err := os.ReadDir(blobs)
				if err != nil || len(files) == 0 {
					t.Fatalf("blobs: %v", err)
				}
				blob := filepath.Join(blobs, files[0].Name())
				bytes, err := os.ReadFile(blob)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(blob, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(blob, []byte("PRIVATE_TOKEN=never-return-this\n"), 0600); err != nil {
					t.Fatal(err)
				}
				capture(c.ID+"-changed-blob", "snapshot_mismatch")
				if err := os.WriteFile(blob, bytes, 0600); err != nil {
					t.Fatal(err)
				}
				capture(c.ID+"-restored-blob", "")
				if err := os.Rename(blobs, blobs+"-saved"); err != nil {
					t.Fatal(err)
				}
				capture(c.ID+"-missing-blobs", "source_missing")
				if err := os.Rename(blobs+"-saved", blobs); err != nil {
					t.Fatal(err)
				}
				capture(c.ID+"-restored-blobs", "")
			}
			if err := runner.Close(); err != nil {
				t.Fatal(err)
			}
			footprintRaw, err := os.ReadFile(filepath.Join(cfg.Logging.FootprintDir, "cks-mcp.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			binaryAfter, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			configAfter, err := os.ReadFile(cfgPath)
			if err != nil {
				t.Fatal(err)
			}
			if guardHash(binaryBefore) != guardHash(binaryAfter) || guardHash(configBefore) != guardHash(configAfter) {
				t.Error("runtime binary/config changed")
			}
			rows = append(rows, map[string]any{"id": c.ID + "-runtime", "binary_sha256": guardHash(binaryBefore), "binary_after_sha256": guardHash(binaryAfter), "config_sha256": guardHash(configBefore), "config_after_sha256": guardHash(configAfter), "config_yaml": string(configBefore), "effective_recall_k": cfg.Retrieval.EffectiveRecallK(), "footprint_sha256": guardHash(footprintRaw), "footprint_jsonl": string(footprintRaw)})
			originalAfter, otherAfter, copyAfter := guardSeal(t, c.Version), guardSeal(t, c.MixedVersion), guardSeal(t, version)
			unchanged := reflect.DeepEqual(originalSeal, originalAfter) && reflect.DeepEqual(otherSeal, otherAfter) && reflect.DeepEqual(copySeal, copyAfter)
			rows = append(rows, map[string]any{"id": c.ID + "-seals", "original_and_copy_payloads_unchanged": unchanged, "original_sha256": originalSeal, "mixed_source_sha256": otherSeal, "copied_sha256": copySeal, "original_after_sha256": originalAfter, "mixed_after_sha256": otherAfter, "copied_after_sha256": copyAfter})
			if !unchanged {
				t.Error("source or retained payload changed outside the temporary mutation")
			}
		})
	}
}

func guardHash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func guardSeal(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("nonregular retained input: %s", p)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = guardHash(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func guardCopy(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		out := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("copy requires regular retained inputs: %s", p)
		}
		// Do not clone SQLite's shared-memory file or an empty WAL. The
		// source's full seal retains both. A nonempty WAL requires a separately
		// checkpointed DEV fixture, since dropping it could lose data.
		if strings.HasSuffix(p, ".db-wal") {
			if info.Size() != 0 {
				t.Fatalf("nonempty WAL requires a checkpointed DEV fixture: %s", p)
			}
			return nil
		}
		if strings.HasSuffix(p, ".db-shm") {
			wal, err := os.Stat(strings.TrimSuffix(p, "-shm") + "-wal")
			if err != nil || wal.Size() != 0 {
				t.Fatalf("SHM requires a verified empty WAL: %s", p)
			}
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, raw, 0600|info.Mode().Perm()&0111)
	})
	if err != nil {
		t.Fatal(err)
	}
}
