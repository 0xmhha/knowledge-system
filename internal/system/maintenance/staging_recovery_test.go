package maintenance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func TestStagingProtocolRequiresReleasedWriterBeforeInterrupting(t *testing.T) {
	for _, protocol := range []string{"owner-socket-mutation-lease-v1", "owner-socket-mutation-lease-staging-v2"} {
		t.Run(protocol, func(t *testing.T) {
			profile, record := durableServiceFixture(t, false)
			options, err := setup.LoadConfigStrict(profile.SetupConfig)
			if err != nil {
				t.Fatal(err)
			}
			record.ExecutionProtocol = protocol
			path := filepath.Join(options.Out, journalDirectory, record.Status.JobID+".json")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			store, err := openJournal(filepath.Dir(path), profile.ProjectID, profile.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.save(record); err != nil {
				t.Fatal(err)
			}
			store.close()
			candidate := filepath.Join(options.Out, record.Request.NewVersion)
			token := strings.Repeat("a", 32)
			directory := ".build-source-" + token
			parent := filepath.Join(candidate, directory)
			if err := os.MkdirAll(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			marker := map[string]any{"schema_version": 1, "project_id": record.Request.ProjectID, "snapshot_id": record.Request.SourcePin.SnapshotID, "dataset_id": record.ExpectedDatasetID, "version": record.Request.NewVersion, "operation_id": record.Status.JobID, "directory": directory, "token": token}
			raw, err := json.Marshal(marker)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{filepath.Join(candidate, "build-staging.json"), filepath.Join(parent, ".owner.json")} {
				if err := os.WriteFile(p, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			witness := filepath.Join(parent, "partial")
			if os.WriteFile(witness, []byte("partial"), 0o600) != nil {
				t.Fatal("partial")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ready, release := make(chan struct{}), make(chan struct{})
			finished := make(chan error, 1)
			go func() {
				finished <- setup.WithDatasetMutationLock(ctx, options.Out, func(context.Context) error {
					close(ready)
					select {
					case <-release:
					case <-ctx.Done():
					}
					return nil
				})
			}()
			select {
			case <-ready:
			case <-ctx.Done():
				t.Fatal("lock readiness")
			}
			service, err := newDurableService([]Profile{profile}, os.Args[0])
			if err != nil {
				close(release)
				t.Fatal(err)
			}
			t.Cleanup(service.Close)
			time.Sleep(100 * time.Millisecond)
			status, err := service.Status(reference(record))
			if err != nil || status.State != "recovering" || status.Terminal {
				close(release)
				t.Fatal("active writer incorrectly completed", err)
			}
			if _, err := os.Stat(witness); err != nil {
				close(release)
				t.Fatal("active staging removed")
			}
			close(release)
			if err := <-finished; err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				status, err = service.Status(reference(record))
				if err != nil {
					t.Fatal(err)
				}
				if protocol == "owner-socket-mutation-lease-v1" {
					time.Sleep(650 * time.Millisecond)
					status, err = service.Status(reference(record))
					if err != nil || status.State != "recovering" {
						t.Fatal("legacy protocol guessed quiescence", err)
					}
					if _, err := os.Stat(witness); err != nil {
						t.Fatal("legacy staging removed")
					}
					break
				}
				if status.Terminal {
					if status.State != "interrupted" {
						t.Fatal("wrong recovery outcome", status.State)
					}
					if _, err := os.Stat(parent); !os.IsNotExist(err) {
						t.Fatal("owned staging survived", err)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("v2 recovery did not settle")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}
