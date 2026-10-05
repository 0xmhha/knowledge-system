package ckgclient

import "strings"

// KeywordFTSQuery encodes a single extracted keyword as FTS data. Ordinary
// identifiers retain their existing query; punctuation and FTS operators must
// not turn a file/symbol name into a column filter or boolean expression.
// BM25Search itself continues to accept explicit FTS expressions.
func KeywordFTSQuery(keyword string) string {
	bare := keyword != ""
	for _, r := range keyword {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			bare = false
			break
		}
	}
	if bare && keyword != "OR" && keyword != "AND" && keyword != "NOT" && keyword != "NEAR" {
		return keyword
	}
	return `"` + strings.ReplaceAll(keyword, `"`, `""`) + `"`
}
