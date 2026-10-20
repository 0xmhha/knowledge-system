package maintenance

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func TestPublicServiceClosePersistsCancellationBeforeStoppingChild(t *testing.T) {
	profile, marker := durableFailingProfile(t)
	options, err := setup.LoadConfigStrict(profile.SetupConfig)
	if err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	if err := os.WriteFile(options.GraphBin, []byte("#!/bin/sh\nprintf started > "+quote(marker)+"\nsleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	service, err := New([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	inspection, err := service.Inspect(context.Background(), ProfileRequest{1, profile.ProjectID, profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	request := requestForInspection(inspection.Profile)
	admitted, err := service.Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child not started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	service.Close()
	again, err := New([]Profile{profile}, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	status, err := again.Status(JobReference{1, request.ProjectID, request.ProfileID, admitted.JobID})
	if err != nil || status.State != "canceled" || !status.Terminal || status.Activated || status.Error == nil || status.Error.Code != "canceled" {
		t.Fatal("clean shutdown cancellation lost", status, err)
	}
	if err := ValidateStatus(status, request, inspection.Profile); err != nil {
		t.Fatal(err)
	}
	replayed, err := again.Build(context.Background(), request)
	if err != nil || !equalJSON(replayed, status) {
		t.Fatal("shutdown replay changed", err)
	}
}
