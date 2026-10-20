package main

import (
	"errors"
	"testing"
)

type v2CodeFixture string

func (v2CodeFixture) Error() string        { return "backend failure detail" }
func (e v2CodeFixture) PublicCode() string { return string(e) }

func TestPublicCLIErrorPreservesTypedV2Codes(t *testing.T) {
	for _, code := range []string{"invalid_request", "embedding_failed", "request_canceled", "request_timeout", "retrieval_failed", "budget_exceeded", "compose_failed", "v2_evidence_failed", "invalid_knowledge_scope", "knowledge_context_failed", "source_missing", "reindex_required"} {
		got := publicCLIError(v2CodeFixture(code))
		if got.Code != code || got.Message == "" || got.Message == "backend failure detail" {
			t.Fatalf("public error %s: %+v", code, got)
		}
	}
}

func TestPublicCLIErrorUnknownTypedCodeIsBounded(t *testing.T) {
	got := publicCLIError(v2CodeFixture("unexpected-code"))
	if got.Code != "operation_failed" || got.Message == "" {
		t.Fatal(got)
	}
}

func TestPublicCLIErrorKeepsLegacyStringClassification(t *testing.T) {
	got := publicCLIError(errors.New("invalid_request"))
	if got.Code != "operation_failed" {
		t.Fatal("legacy error classification changed", got)
	}
	legacy := publicCLIError(errors.New("durability_uncertain"))
	v2 := publicCLIError(v2CodeFixture("durability_uncertain"))
	if legacy.Code != v2.Code || legacy.Message == v2.Message {
		t.Fatal("output file and current-pointer uncertainty conflated", legacy, v2)
	}
}
