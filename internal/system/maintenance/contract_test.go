package maintenance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func designFixtures(t *testing.T) []struct {
	ID, Kind, Expected string
	Data               json.RawMessage
} {
	t.Helper()
	raw, err := os.ReadFile("../../../system/contracts/versioned-maintenance/v1/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Cases []struct {
			ID, Kind, Expected string
			Data               json.RawMessage
		}
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures.Cases
}

func TestBuildAdmissionAgreesWithDesignFixtures(t *testing.T) {
	for _, fixture := range designFixtures(t) {
		if fixture.Kind != "request" {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			var data struct {
				Request json.RawMessage
				Profile ProfileContext
			}
			if err := json.Unmarshal(fixture.Data, &data); err != nil {
				t.Fatal(err)
			}
			var request BuildRequest
			err := Decode(data.Request, &request)
			if err == nil {
				err = request.Validate()
			}
			if err == nil {
				err = checkRequest(request, data.Profile, true)
			}
			actual := "accept"
			if err != nil {
				actual = ErrorResult(err).Code
			}
			if actual != fixture.Expected {
				t.Fatalf("want %s, got %s", fixture.Expected, actual)
			}
		})
	}
}

func TestDecodeRejectsMissingDuplicateAndCaseAliasedFields(t *testing.T) {
	for name, raw := range map[string]string{
		"missing":    `{"schema_version":1,"project_id":"a"}`,
		"duplicate":  `{"schema_version":1,"project_id":"a","project_id":"b","profile_id":"p"}`,
		"case-alias": `{"schema_version":1,"project_id":"a","profile_id":"p","PROJECT_ID":"b"}`,
		"null":       `null`, "trailing": `{"schema_version":1,"project_id":"a","profile_id":"p"}{}`,
		"decimal":   `{"schema_version":1.0,"project_id":"a","profile_id":"p"}`,
		"oversized": strings.Repeat(" ", 65537), "path": `{"schema_version":1,"project_id":"a","profile_id":"p","path":"/private/other"}`,
	} {
		t.Run(name, func(t *testing.T) {
			var request ProfileRequest
			if Decode([]byte(raw), &request) == nil {
				t.Fatal("invalid JSON contract accepted")
			}
		})
	}
	var request ProfileRequest
	if err := Decode([]byte(`{"schema_version":1,"project_id":"a","profile_id":"p"}`), &request); err != nil {
		t.Fatal(err)
	}
}

func TestRequestDigestMatchesDesignCanonicalEncoding(t *testing.T) {
	for _, fixture := range designFixtures(t) {
		if fixture.ID != "ID-replay" {
			continue
		}
		var data struct {
			Request BuildRequest
			Record  struct {
				RequestDigest string `json:"request_digest"`
			}
		}
		if err := json.Unmarshal(fixture.Data, &data); err != nil {
			t.Fatal(err)
		}
		actual, err := Digest(data.Request, "cks.dataset-build.v1")
		if err != nil || actual != data.Record.RequestDigest {
			t.Fatalf("canonical fingerprint differs: %s %v", actual, err)
		}
		return
	}
	t.Fatal("missing canonical fixture")
}

func memoryJob(t *testing.T) (*Service, JobReference, context.Context) {
	t.Helper()
	s, err := New(nil, "unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ctx, cancel := context.WithCancel(context.Background())
	ref := JobReference{SchemaVersion: 1, ProjectID: "a", ProfileID: "p", JobID: "job-one"}
	s.jobs[ref.JobID] = &job{request: BuildRequest{ProjectID: "a", ProfileID: "p"}, cancel: cancel,
		status: JobStatus{SchemaVersion: 1, ProjectID: "a", ProfileID: "p", JobID: ref.JobID, State: "running", Phase: "capture", Revision: 1, Gates: map[string]string{"graph": "not_run"}}}
	t.Cleanup(cancel)
	return s, ref, ctx
}

func TestScopedStatusAndCancelCannotUseAnotherProjectsJob(t *testing.T) {
	s, ref, ctx := memoryJob(t)
	foreign := ref
	foreign.ProjectID = "b"
	if _, err := s.Status(foreign); ErrorResult(err).Code != "job_unknown" {
		t.Fatal("foreign status accepted")
	}
	foreign = ref
	foreign.ProfileID = "different"
	if _, err := s.Cancel(foreign); ErrorResult(err).Code != "job_unknown" || ctx.Err() != nil {
		t.Fatal("foreign cancel changed job")
	}
	result, err := s.Cancel(ref)
	if err != nil || result.Decision != "accepted" || result.Status.State != "cancel_requested" || ctx.Err() != context.Canceled {
		t.Fatal("cancellation was not requested")
	}
	result, err = s.Cancel(ref)
	if err != nil || result.Decision != "already_requested" {
		t.Fatal("cancel not idempotent")
	}
}

func TestPublicationWinsCancelWithoutRollingBack(t *testing.T) {
	s, ref, ctx := memoryJob(t)
	j := s.jobs[ref.JobID]
	j.mu.Lock()
	j.status.Activated, j.status.Terminal, j.status.State = true, true, "succeeded"
	j.mu.Unlock()
	result, err := s.Cancel(ref)
	if err != nil || result.Decision != "too_late" || ctx.Err() != nil || !result.Status.Activated {
		t.Fatal("published job was canceled")
	}
}

func TestConcurrentCancelAndDetachedStatus(t *testing.T) {
	s, ref, _ := memoryJob(t)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Go(func() {
			_, _ = s.Cancel(ref)
			value, err := s.Status(ref)
			if err == nil {
				value.Gates["graph"] = "foreign"
			}
		})
	}
	group.Wait()
	status, err := s.Status(ref)
	if err != nil || status.Gates["graph"] != "not_run" || status.State != "cancel_requested" {
		t.Fatal("status map escaped job ownership")
	}
}

func TestDatasetOwnerCannotBeReplacedOrSymlinked(t *testing.T) {
	root := t.TempDir()
	if err := claimRoot(root, "project-a"); err != nil {
		t.Fatal(err)
	}
	if err := claimRoot(root, "project-a"); err != nil {
		t.Fatal(err)
	}
	if err := claimRoot(root, "project-b"); ErrorResult(err).Code != "project_mismatch" {
		t.Fatal("foreign root accepted")
	}
	other := t.TempDir()
	if err := os.Symlink(filepath.Join(root, ownerFile), filepath.Join(other, ownerFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := ownsRoot(other, "project-a"); err == nil {
		t.Fatal("linked owner accepted")
	}
}

func TestDigestPreservesExactLargeIntegerIdentity(t *testing.T) {
	a, err := Digest(map[string]uint64{"revision": 9007199254740992}, "owned.test")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Digest(map[string]uint64{"revision": 9007199254740993}, "owned.test")
	if err != nil || a == b {
		t.Fatal("integer precision merged distinct records")
	}
}
