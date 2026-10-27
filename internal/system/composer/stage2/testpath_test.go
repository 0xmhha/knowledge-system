package stage2

import "testing"

func TestIsTestPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		file string
		want bool
	}{
		// Go production files — not test
		{"main.go", false},
		{"handler.go", false},
		{"core/txpool/legacypool.go", false},
		{"cmd/gstable/main.go", false},

		// Go test files
		{"handler_test.go", true},
		{"core/txpool/legacypool_test.go", true},
		{"internal/composer/stage2/merge_test.go", true},

		// TypeScript production
		{"app.ts", false},
		{"components/Button.tsx", false},

		// TypeScript test files
		{"handler.test.ts", true},
		{"handler.spec.ts", true},
		{"components/Button.test.tsx", true},
		{"components/Button.spec.tsx", true},

		// JavaScript production
		{"index.js", false},
		{"utils.jsx", false},

		// JavaScript test files
		{"index.test.js", true},
		{"index.spec.js", true},
		{"utils.test.jsx", true},
		{"utils.spec.jsx", true},

		// Solidity production
		{"contracts/Token.sol", false},
		{"contracts/Vault.sol", false},

		// Solidity test files — *.t.sol convention
		{"contracts/Token.t.sol", true},
		{"contracts/Vault.t.sol", true},

		// Solidity test files — "test" or "tests" path segment
		{"test/Token.sol", true},
		{"tests/Token.sol", true},
		{"src/test/Token.sol", true},
		{"src/tests/Token.sol", true},
		{"contracts/test/Token.sol", true},

		// Edge cases
		{"", false},
		// "test" as filename, not segment — no match for ".go" suffix
		{"test.go", false},
		// "tests.go" — not a test file, suffix is not "_test.go"
		{"tests.go", false},
		// A file that merely contains "test" in its name (not segment, not suffix)
		{"contextual.go", false},
		{"protest.go", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			got := isTestPath(tc.file)
			if got != tc.want {
				t.Errorf("isTestPath(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestPromptAsksAboutTests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		prompt string
		want   bool
	}{
		{"서버 스트리밍 시험이 가끔 실패해. 고치고 싶어", true},
		{"CDS e2e 테스트 시간 제한을 늘리고 싶어", true},
		{"fix the flaky test in end2end", true},
		{"Add tests for the retry policy", true},
		{"퍼저에서 쓸 함수를 공개하고 싶어", true},
		{"add a benchmark for the encoder", true},
		{"xDS 클라이언트가 리소스 수를 지표로 보고하게 하고 싶어", false},
		{"fix the race in attestation handling", false},
		// Substrings of longer words are not a cue.
		{"update the contest and latest config", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := promptAsksAboutTests(tc.prompt); got != tc.want {
			t.Errorf("promptAsksAboutTests(%q) = %v, want %v", tc.prompt, got, tc.want)
		}
	}
}
