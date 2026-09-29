package semanticcli

import (
	"encoding/json"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/spf13/cobra"
)

// lookup-term is an exact vocabulary review aid. It revalidates the active
// dataset before using the normalized SQLite index and keeps all ambiguous
// concepts; it never rewrites a natural-language query or asserts meaning.
func newLookupTermCmd() *cobra.Command {
	var project, repo, graph, vector, storePath, lang, term string
	cmd := &cobra.Command{Use: "lookup-term", Short: "Find every reviewed or proposed concept for an exact language term",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			active, err := store.CurrentAligned(cmd.Context(), project, repo, graph, vector)
			if err != nil {
				return err
			}
			hits, err := store.LookupTerm(cmd.Context(), project, active.Snapshot().DatasetID, lang, term)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(struct {
				Snapshot   semantic.Snapshot           `json:"snapshot"`
				Language   string                      `json:"language"`
				Term       string                      `json:"term"`
				Candidates []semantic.ConceptCandidate `json:"candidates"`
			}{Snapshot: active.Snapshot(), Language: lang, Term: term, Candidates: hits})
		},
	}
	for _, flag := range []struct {
		name   string
		target *string
		usage  string
	}{
		{"project-id", &project, "active project identifier"},
		{"repo", &repo, "Git repository containing the cited commit"},
		{"graph", &graph, "CKG data directory"},
		{"vector", &vector, "CKV data directory"},
		{"store", &storePath, "CKS semantic SQLite path"},
		{"lang", &lang, "term language tag"},
		{"term", &term, "exact concept term (case insensitive)"},
	} {
		cmd.Flags().StringVar(flag.target, flag.name, "", flag.usage)
		_ = cmd.MarkFlagRequired(flag.name)
	}
	return cmd
}
