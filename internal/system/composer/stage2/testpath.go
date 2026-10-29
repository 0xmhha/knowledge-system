package stage2

import (
	"regexp"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/system/testpath"
)

// testDemotionFactor is the multiplier applied to a test-file citation's
// final RRF score when the active intent is not test-oriented. A value of
// 0.25 means a test file must accumulate four times the raw RRF evidence
// of a production file to outrank it — enough to keep tests accessible
// lower in the evidence pack without hiding them entirely.
const testDemotionFactor = 0.25

// TestDemotionFactor exports testDemotionFactor for Stage 3, which applies
// the same demotion to test-file neighbors when Stage 2 demoted tests.
const TestDemotionFactor = testDemotionFactor

// isTestPath reports whether file is a test or test-only support file. It
// delegates to pkg/testpath.IsTest, the single source of truth shared with
// the exclude_tests query filter, so graph and vector citations are
// classified consistently without re-indexing.
func isTestPath(file string) bool {
	return testpath.IsTest(file)
}

// testPromptWords are English words that make a prompt about tests. They
// match as whole words, so "contest" or "latest" do not count.
var testPromptWords = regexp.MustCompile(`(?i)\b(tests?|flaky|e2e|fuzz|benchmark)\b`)

// testPromptWordsKo are Korean words for tests. Korean attaches particles
// to nouns ("시험이", "테스트를"), so these match as substrings.
var testPromptWordsKo = []string{"시험", "테스트", "퍼저", "벤치마크"}

// promptAsksAboutTests reports whether the prompt itself is about tests. When
// it is, test files are what the user wants, so Stage 2 does not demote them
// even if the intent classifier did not return IntentTestAdd. The cue reads
// the prompt, not the embedding model, so it does not move with a model's
// similarity scale (measured 2026-10-27 on grpc-go: 21 of 22 test_add prompts
// carry a cue, 2 of 78 other prompts do; the classifier's nearest intent was
// test_add for 3 to 5 of the 22).
func promptAsksAboutTests(prompt string) bool {
	if testPromptWords.MatchString(prompt) {
		return true
	}
	for _, w := range testPromptWordsKo {
		if strings.Contains(prompt, w) {
			return true
		}
	}
	return false
}
