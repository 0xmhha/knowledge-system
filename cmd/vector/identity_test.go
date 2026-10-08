package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestIdentityCommandMatchesMockSpace(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--embedder", "mock", "identity"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var response struct {
		EmbeddingIdentity struct {
			Model    string `json:"model"`
			Dim      int    `json:"dim"`
			Checksum string `json:"checksum"`
		} `json:"embedding_identity"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.EmbeddingIdentity.Model == "" || response.EmbeddingIdentity.Dim <= 0 || response.EmbeddingIdentity.Checksum == "" {
		t.Fatalf("incomplete pre-build identity: %+v", response)
	}
}
