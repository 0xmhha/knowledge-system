package setup

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func retainedVersionForRequest(t *testing.T) (string, CapturedSource) {
	t.Helper()
	root, version := t.TempDir(), t.TempDir()
	for name, text := range map[string]string{"a.md": "alpha\n", "b.md": "beta\n", "unused.md": "uncited\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	captured, err := CaptureSource(CaptureOptions{Root: root, Out: version, ProjectID: "files", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	writePinnedMockManifests(t, version, root, "", captured.Identity)
	if _, err := PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	return version, captured
}

func alterRetainedBlob(t *testing.T, captured CapturedSource, path string) {
	t.Helper()
	for _, f := range captured.Files {
		if f.Path == path {
			blob := filepath.Join(captured.BlobDir, f.SHA256)
			if err := os.Chmod(blob, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(blob, []byte("altered\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no retained file %s", path)
}

func TestRequestVerificationIsSharedOnlyWithinOneRequest(t *testing.T) {
	version, captured := retainedVersionForRequest(t)
	ctx, v, err := BeginRequestVerification(context.Background(), version)
	if err != nil || v == nil {
		t.Fatalf("begin: %v %v", v, err)
	}
	want, err := InspectVersionIdentity(version)
	if err != nil || !reflect.DeepEqual(v.Identity(), want) {
		t.Fatalf("request identity differs: %+v %+v %v", v.Identity(), want, err)
	}
	// Damage to an unread blob is invisible to reads that share the request
	// verification but is caught by an independent read and by Finish.
	alterRetainedBlob(t, captured, "unused.md")
	if buf, _, _, err := ReadRetainedFileContext(ctx, version, "repo", "a.md"); err != nil || string(buf) != "alpha\n" {
		t.Fatalf("request read did not reuse verification: %q %v", buf, err)
	}
	if _, _, _, err := ReadRetainedFile(version, "repo", "a.md"); err == nil {
		t.Fatal("read outside the request reused its verification")
	}
	if err := v.Finish(ctx); err == nil {
		t.Fatal("finish accepted an archive damaged during the request")
	}
	if _, _, _, err := ReadRetainedFileContext(ctx, version, "repo", "a.md"); err == nil {
		t.Fatal("read after finish reused the ended verification")
	}
}

func TestRequestVerificationStillHashesReadBlobs(t *testing.T) {
	version, captured := retainedVersionForRequest(t)
	ctx, v, err := BeginRequestVerification(context.Background(), version)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	alterRetainedBlob(t, captured, "b.md")
	if buf, _, _, err := ReadRetainedFileContext(ctx, version, "repo", "b.md"); err == nil {
		t.Fatalf("damaged blob read inside a request: %q", buf)
	}
	if out, _, err := ReadRetainedFiles(ctx, version, []RetainedFile{{"repo", "a.md"}, {"repo", "b.md"}}); err == nil || out != nil {
		t.Fatalf("batch returned a damaged blob: %v", err)
	}
}

func TestRequestVerificationScopeAndLifetime(t *testing.T) {
	version, _ := retainedVersionForRequest(t)
	other, otherCaptured := retainedVersionForRequest(t)
	ctx, v, err := BeginRequestVerification(context.Background(), version)
	if err != nil {
		t.Fatal(err)
	}
	alterRetainedBlob(t, otherCaptured, "unused.md")
	if _, _, _, err := ReadRetainedFileContext(ctx, other, "repo", "a.md"); err == nil {
		t.Fatal("another version reused this request's verification")
	}
	if err := v.Finish(ctx); err != nil {
		t.Fatalf("clean finish: %v", err)
	}

	ctx, v, err = BeginRequestVerification(context.Background(), version)
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	if id := requestVerifiedIdentity(ctx, version); id != nil {
		t.Fatal("closed verification still served an identity")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := InspectVersionIdentityContext(cancelled, version); err == nil {
		t.Fatal("cancelled request returned an identity")
	}

	legacyCtx, legacy, err := BeginRequestVerification(context.Background(), t.TempDir())
	if err != nil || legacy != nil || legacyCtx.Value(requestVerificationKey{}) != nil {
		t.Fatalf("legacy version opened a verification: %v %v", legacy, err)
	}
	legacy.Close()
}
