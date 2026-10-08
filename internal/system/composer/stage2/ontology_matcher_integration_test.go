package stage2

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"gopkg.in/yaml.v3"
)

// Exercise the real parser, immutable store, matcher and retained evidence
// path together. The engine manifests/hits are coordinate-only controls;
// there are no code edges, live retrieval calls or human-reviewed meanings.
func TestStoredSharedAliasPreservesBaselineAndRetainedEvidence(t *testing.T) {
	ctx := context.Background()
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: fixture-secret\n    description: owned test marker\n    pattern: SECRET\n    action: drop\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	cleaner, err := sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, conceptCount := range []int{2, 9} {
		t.Run(fmt.Sprintf("concepts-%d", conceptCount), func(t *testing.T) {
			root, version := t.TempDir(), t.TempDir()
			pack := semantic.OntologyPack{Version: 1, ProjectID: "alias-control", Domain: "owned ambiguity control", CompetencyQuestions: []string{"Which meanings share one alias?"}}
			for i := 0; i < conceptCount; i++ {
				id := fmt.Sprintf("meaning-%02d", i)
				pack.Concepts = append(pack.Concepts, semantic.Concept{ID: id, Kind: "entity", Definition: "An owned proposed meaning.", Includes: []string{"This fixture"}, Excludes: []string{"Actual pilot approval"}, Status: semantic.StatusProposed,
					Terms: []semantic.Term{{Lang: "en", Value: id, Preferred: true}, {Lang: "en", Value: "shared", Preferred: false}, {Lang: "ko", Value: "의미" + fmt.Sprint(i), Preferred: true}, {Lang: "ko", Value: "공유어", Preferred: false}}})
			}
			body, err := yaml.Marshal(pack)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "ontology.yaml"), body, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("public fixture evidence\nSECRET owned marker\nthird public line\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			identity := aliasControlVersion(t, root, version)
			snapshot := semantic.Snapshot{ProjectID: identity.Source.ProjectID, DatasetID: identity.DatasetID, SnapshotID: identity.Source.SnapshotID, SourceMode: "snapshot-only"}
			projection, _, err := semantic.ExtractOntology(snapshot, "ontology.yaml", body)
			if err != nil {
				t.Fatal(err)
			}
			storePath := filepath.Join(t.TempDir(), "semantic.sqlite")
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			graphDir, vectorDir := filepath.Join(version, "graph"), filepath.Join(version, "vector")
			if err := store.PutAlignedRetained(ctx, projection, root, graphDir, vectorDir, version); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			// The immutable retained archive, rather than live source bytes,
			// must supply both vocabulary evidence and sanitized bodies.
			if err := os.RemoveAll(root); err != nil {
				t.Fatal(err)
			}
			store, err = semantic.OpenStore(storePath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			for _, term := range []struct{ lang, value string }{{"en", "SHARED"}, {"ko", "공유어"}} {
				lookup, err := store.LookupTerm(ctx, snapshot.ProjectID, snapshot.DatasetID, term.lang, term.value)
				if err != nil || len(lookup) != conceptCount {
					t.Fatalf("stored alias %s: %+v %v", term.value, lookup, err)
				}
			}
			resolver, err := store.LoadAlignedRetained(ctx, snapshot.ProjectID, snapshot.DatasetID, root, graphDir, vectorDir, version)
			if err != nil {
				t.Fatal(err)
			}
			for _, query := range []string{"shared", "공유어를 확인"} {
				matches, err := resolver.MatchConcepts(query, 1)
				if err != nil || len(matches) != conceptCount {
					t.Fatalf("cutoff collapsed shared alias %q: %+v %v", query, matches, err)
				}
				for i, c := range matches {
					if c.ConceptID != fmt.Sprintf("meaning-%02d", i) || c.Status != semantic.StatusProposed || c.Score != 0.4 || len(resolver.VerifiedImplementations(c.ConceptID)) != 0 {
						t.Fatalf("unexpected proposed interpretation: %+v", c)
					}
				}
			}
			hits := []contract.Hit{}
			for line := 1; line <= 3; line++ {
				hits = append(hits, contract.Hit{Citation: contract.Citation{File: "README.md", StartLine: line, EndLine: line}})
			}
			cfg := DefaultConfig()
			cfg.MaxCitations = 2
			plain, err := New(&ckgclient.Fake{}, WithConfig(cfg))
			if err != nil {
				t.Fatal(err)
			}
			opted, err := New(&ckgclient.Fake{}, WithConfig(cfg), WithOntologyProvider(providerFunc(func(context.Context) (OntologyResolver, error) { return resolver, nil }), 0.2, time.Second))
			if err != nil {
				t.Fatal(err)
			}
			for _, query := range []string{"shared", "공유어를 확인", "meaning-00", "unshared", "unknown"} {
				t.Run(query, func(t *testing.T) {
					base, err := plain.Search(ctx, query, nil, hits, contract.IntentBugFix)
					if err != nil {
						t.Fatal(err)
					}
					out, err := opted.Search(ctx, query, nil, hits, contract.IntentBugFix)
					if err != nil || !reflect.DeepEqual(out.Citations, base.Citations) || len(out.Citations) != 2 || out.Ontology == nil || out.Ontology.AppliedRelations != 0 || out.Ontology.BoostedCitations != 0 {
						t.Fatalf("actual matcher changed baseline: %+v %v", out, err)
					}
					wantCount, wantState := 0, "no_match"
					if query == "meaning-00" {
						wantCount, wantState = 1, "active"
					} else if query == "shared" || strings.HasPrefix(query, "공유어") {
						wantCount, wantState = conceptCount, "active"
						if conceptCount > 8 {
							wantState = "budget_exceeded"
						}
					}
					if out.Ontology.MatchedConcepts != wantCount || out.Ontology.State != wantState {
						t.Fatalf("unexpected diagnostic: %+v", out.Ontology)
					}
					if wantState == "budget_exceeded" {
						if len(out.ConceptCandidates) != 0 || out.Ontology.Reason != "concept_limit" {
							t.Fatal("overflow selected one interpretation")
						}
					} else if len(out.ConceptCandidates) != wantCount {
						t.Fatal("matcher interpretations were lost")
					}
					refs := make([]contract.Citation, len(base.Citations))
					for i, c := range base.Citations {
						refs[i] = c.Citation
					}
					basePack, err := evidencev2.Build(ctx, version, query, refs, cleaner)
					if err != nil {
						t.Fatal(err)
					}
					optionalPack, err := evidencev2.Build(ctx, version, query, refs, cleaner)
					if err != nil || !reflect.DeepEqual(basePack, optionalPack) || len(basePack.Bodies) != 1 || basePack.EvidenceState != "partial" {
						t.Fatalf("retained/sanitized baseline differs: %+v %v", optionalPack, err)
					}
					optionalPack.Metadata.Ontology = out.Ontology
					if err := evidencev2.Stamp(&optionalPack); err != nil {
						t.Fatal(err)
					}
					if err := evidencev2.Verify(optionalPack); err != nil || optionalPack.Metadata.IntegrityHash == basePack.Metadata.IntegrityHash || !reflect.DeepEqual(optionalPack.Citations, basePack.Citations) || !reflect.DeepEqual(optionalPack.Bodies, basePack.Bodies) || optionalPack.Coordinates != basePack.Coordinates {
						t.Fatalf("optional diagnostic disturbed evidence: %v", err)
					}
					optionalPack.Metadata.Ontology.MatchedConcepts++
					if err := evidencev2.Verify(optionalPack); err == nil {
						t.Fatal("tampered diagnostic accepted")
					}
					t.Logf("owned alias control: concepts=%d query=%q matched=%d state=%s unchanged capped citations=2 sanitized bodies=1 tamper refused=true", conceptCount, query, wantCount, wantState)
				})
			}
			if _, err := store.Current(ctx, snapshot.ProjectID); err != sql.ErrNoRows {
				t.Fatalf("query activated semantic current: %v", err)
			}
		})
	}
}

func aliasControlVersion(t *testing.T, root, version string) setup.DatasetIdentity {
	t.Helper()
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: root, Out: version, ProjectID: "alias-control", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "alias-coordinate-control")
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"graph", "vector"} {
		dir := filepath.Join(version, side)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"src_root": root, "src_commit": "", "source_mode": "snapshot-only", "schema_version": "1.23", "graph_digest": strings.Repeat("a", 64),
			"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID, "dataset_id": expected.DatasetID, "file_manifest_digest": captured.Identity.FileManifestDigest, "capture_policy_digest": captured.Identity.CapturePolicyDigest,
			"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space", "sources": map[string]any{"ckg": map[string]any{"graph_digest": strings.Repeat("a", 64), "src_commit": ""}}}
		body, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := setup.PublishCandidateIdentity(version, captured.Identity, "alias-coordinate-control")
	if err != nil {
		t.Fatal(err)
	}
	return identity
}
