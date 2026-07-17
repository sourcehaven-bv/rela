package markdown

import (
	"os"
	"testing"
)

// loadCodeSpanGrowthFixture returns the real ticket body (found via the sync
// e2e) on which the goldmark-markdown renderer pads whitespace inside an inline
// code span by one space on every render pass — so formatOnce never settles.
func loadCodeSpanGrowthFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/idempotency_codespan_growth.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(raw)
}

// TestFormatMarkdown_CodeSpanGrowthIsIdempotent pins the fix: FormatMarkdown is
// idempotent even on the runaway-whitespace body. Before the fix,
// FormatMarkdown(FormatMarkdown(x)) grew by ~97 bytes.
func TestFormatMarkdown_CodeSpanGrowthIsIdempotent(t *testing.T) {
	body := loadCodeSpanGrowthFixture(t)

	once := FormatMarkdown(body)
	twice := FormatMarkdown(once)
	if once != twice {
		t.Fatalf("FormatMarkdown not idempotent on the runaway-whitespace body: "+
			"len(once)=%d len(twice)=%d (diff=%d)", len(once), len(twice), len(twice)-len(once))
	}
	if thrice := FormatMarkdown(twice); thrice != twice {
		t.Fatalf("FormatMarkdown drifted on a third pass: diff=%d", len(thrice)-len(twice))
	}
}

// TestFormatMarkdown_CodeSpanGrowthConvergesCrossRepresentation is the property
// the canonical content hash (and thus sync) depends on: the raw body and an
// already-reflowed body must canonicalize to the SAME value, so the same content
// hashes equally across fsstore (reflowed on disk) and pgstore (raw). Before the
// fix these landed on different non-converged fixed points.
func TestFormatMarkdown_CodeSpanGrowthConvergesCrossRepresentation(t *testing.T) {
	body := loadCodeSpanGrowthFixture(t)

	fromRaw := FormatMarkdown(body)
	fromReflowed := FormatMarkdown(formatOnce(body, DefaultLineWidth))
	if fromRaw != fromReflowed {
		t.Fatalf("cross-representation canonicalization mismatch: "+
			"raw→%d bytes, reflowed→%d bytes (must be equal for the cross-backend hash to match)",
			len(fromRaw), len(fromReflowed))
	}
}

// TestNormalizeCodeSpanWhitespace covers the helper directly: runaway padding
// before a closing backtick collapses to one space; a single space is preserved.
func TestNormalizeCodeSpanWhitespace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"`code      `", "`code `"},                      // 6 spaces before ` → 1
		{"`code `", "`code `"},                           // single space preserved
		{"`code`", "`code`"},                             // no trailing space untouched
		{"`a` and `b   `", "`a` and `b `"},               // only the padded span collapses
		{"plain text   here", "plain text   here"},       // spaces NOT before a backtick untouched
		{"``a `b`   ``", "``a `b` ``"},                   // nested-backtick span: padding before `` collapses
		{"end of span   `\nrest", "end of span `\nrest"}, // padding before a wrapped continuation backtick
	}
	for _, tc := range cases {
		if got := normalizeCodeSpanWhitespace(tc.in); got != tc.want {
			t.Errorf("normalizeCodeSpanWhitespace(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
