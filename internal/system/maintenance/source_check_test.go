package maintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func sourceCheckFixture(t *testing.T, body string) *SourceCheck {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	sh, err = filepath.EvalSymlinks(sh)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "check.sh")
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	pins := map[string]string{}
	for _, path := range []string{sh, script} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		pins[path] = hex.EncodeToString(h[:])
	}
	return &SourceCheck{Command: []string{sh, script}, PinnedFiles: pins, TimeoutSeconds: 5}
}

func TestSourceCheckValidatesOperatorOnlyShape(t *testing.T) {
	for _, mode := range []string{"empty", "relative", "unbound", "bad-hash", "zero-timeout", "long-timeout", "nul-argument", "relative-pin"} {
		t.Run(mode, func(t *testing.T) {
			check := sourceCheckFixture(t, "exit 0\n")
			switch mode {
			case "empty":
				check.Command = nil
			case "relative":
				check.Command[0] = "sh"
			case "unbound":
				delete(check.PinnedFiles, check.Command[0])
			case "bad-hash":
				check.PinnedFiles[check.Command[0]] = "bad"
			case "zero-timeout":
				check.TimeoutSeconds = 0
			case "long-timeout":
				check.TimeoutSeconds = 61
			case "nul-argument":
				check.Command = append(check.Command, "bad\x00arg")
			case "relative-pin":
				check.PinnedFiles["script"] = check.PinnedFiles[check.Command[0]]
			}
			if check.Validate() == nil {
				t.Fatal("invalid operator check accepted")
			}
		})
	}
}

func TestSourceCheckRunsBoundArgvAndDiscardsOutput(t *testing.T) {
	check := sourceCheckFixture(t, "printf 'untrusted stdout\\n'\nprintf 'untrusted stderr\\n' >&2\n[ \"$PYTHONDONTWRITEBYTECODE\" = 1 ]\n")
	if err := runSourceCheck(context.Background(), check, os.Args[0]); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCheckFailureAndMissingFileAreRefused(t *testing.T) {
	check := sourceCheckFixture(t, "exit 3\n")
	if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "source_stale" {
		t.Fatalf("want source_stale: %v", err)
	}
	if err := os.Remove(check.Command[1]); err != nil {
		t.Fatal(err)
	}
	if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "profile_stale" {
		t.Fatalf("want profile_stale: %v", err)
	}
}

func TestSourceCheckChangedScriptNeverExecutes(t *testing.T) {
	check := sourceCheckFixture(t, "exit 0\n")
	marker := filepath.Join(filepath.Dir(check.Command[1]), "unwanted")
	if err := os.WriteFile(check.Command[1], []byte("touch '"+marker+"'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "profile_stale" {
		t.Fatalf("changed check ran: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("unpinned command executed")
	}
}

func TestSourceCheckRejectsLinksAndDirectories(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			check := sourceCheckFixture(t, "exit 0\n")
			original := check.Command[1]
			moved := original + ".old"
			if err := os.Rename(original, moved); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(moved, original)
			case "hardlink":
				err = os.Link(moved, original)
			case "directory":
				err = os.Mkdir(original, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "profile_stale" {
				t.Fatalf("linked check accepted: %v", err)
			}
		})
	}
}

func TestSourceCheckPinsStillMatchAfterExecution(t *testing.T) {
	check := sourceCheckFixture(t, "printf changed >> \"$0\"\nexit 0\n")
	if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "profile_stale" {
		t.Fatalf("self-mutating check accepted: %v", err)
	}
}

func TestSourceCheckHasDeadlineAndStopsLateDescendant(t *testing.T) {
	// Only a descendant surviving the returned deadline can create the marker.
	// A fixed sleep cannot distinguish a write before return from a late write
	// when the process scheduler or an instrumented guardian is delayed.
	check := sourceCheckFixture(t, "(while [ ! -f \"$2\" ]; do /bin/sleep 0.01; done; printf orphan > \"$1\") &\nprintf '%s\\n' \"$!\" > \"$3\"\nwait\n")
	marker := filepath.Join(filepath.Dir(check.Command[1]), "late")
	release := marker + ".release"
	ready := marker + ".ready"
	check.Command = append(check.Command, marker, release, ready)
	check.TimeoutSeconds = 1
	if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "deadline_exceeded" {
		t.Fatalf("timeout accepted: %v", err)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatalf("descendant was not entered before the deadline: %v", err)
	}
	if err := os.WriteFile(release, []byte("check returned"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("timed-out check left a writer")
	}
}

func TestSourceCheckCanceledContextCannotExecute(t *testing.T) {
	check := sourceCheckFixture(t, "exit 0\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if runSourceCheck(ctx, check, os.Args[0]) != context.Canceled {
		t.Fatal("canceled check executed")
	}
}

func TestSourceCheckNilKeepsOriginalProfileContract(t *testing.T) {
	if err := runSourceCheck(context.Background(), nil, ""); err != nil {
		t.Fatal(err)
	}
	profile, _ := durableServiceFixture(t, true)
	legacy := map[string]any{"id": profile.ID, "project_id": profile.ProjectID, "setup_config": profile.SetupConfig,
		"allowed_activation": profile.AllowedActivation, "timeout_seconds": profile.TimeoutSeconds}
	before, err := Digest(legacy, "profile")
	if err != nil {
		t.Fatal(err)
	}
	profile.SourceCheck = nil
	after, err := Digest(profile, "profile")
	if err != nil || before != after {
		t.Fatal("optional field changed legacy profile digest")
	}
}

func TestSourceCheckRejectsOversizedPinnedFileBeforeReading(t *testing.T) {
	check := sourceCheckFixture(t, "exit 0\n")
	file, err := os.OpenFile(check.Command[1], os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((128 << 20) + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runSourceCheck(context.Background(), check, os.Args[0]); ErrorResult(err).Code != "profile_stale" {
		t.Fatalf("oversized pin accepted: %v", err)
	}
}

func TestSourceCheckDoesNotBlockPreviouslyAdmittedReplay(t *testing.T) {
	profile, record := durableServiceFixture(t, true)
	check := sourceCheckFixture(t, "exit 2\n")
	profile.SourceCheck = check
	service, err := newDurableService([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	job, err := service.Build(context.Background(), record.Request)
	if err != nil || !equalJSON(job, record.Status) {
		t.Fatalf("historical replay ran a changed source check: %v", err)
	}
	ref := ProfileRequest{SchemaVersion: 1, ProjectID: profile.ProjectID, ProfileID: profile.ID}
	if _, err := service.Inspect(context.Background(), ref); ErrorResult(err).Code != "source_stale" {
		t.Fatalf("new inspect skipped live guard: %v", err)
	}
}

func TestSourceCheckCannotBeChosenThroughToolArguments(t *testing.T) {
	var request ProfileRequest
	raw := []byte(`{"schema_version":1,"project_id":"owned","profile_id":"registered","source_check":{"command":["/bin/sh"]}}`)
	if Decode(raw, &request) == nil {
		t.Fatal("public request selected an executable")
	}
}
