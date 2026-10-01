package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestPublicCLIErrorUsesStableCodeWithoutLeakingNestedDetails(t *testing.T) {
	for _, tc := range []struct{ input, code string }{
		{"pack_lock_mismatch: /tmp/stage/secret.yaml contains API_KEY=value", "pack_lock_mismatch"},
		{"promote candidate: snapshot_mismatch: /private/tmp/staging", "snapshot_mismatch"},
		{"open /tmp/stage/secret.yaml: permission denied", "operation_failed"},
	} {
		body, err := json.Marshal(publicCLIError(fmt.Errorf("%s", tc.input)))
		if err != nil {
			t.Fatal(err)
		}
		var got cliError
		if err := json.Unmarshal(body, &got); err != nil || got.Code != tc.code || got.Message == "" ||
			strings.Contains(string(body), "secret.yaml") || strings.Contains(string(body), "API_KEY") ||
			strings.Contains(string(body), "/tmp/") || strings.Contains(string(body), "staging") {
			t.Fatalf("unsafe CLI error: %s, %v", body, err)
		}
	}
}
