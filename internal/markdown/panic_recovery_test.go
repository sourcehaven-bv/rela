package markdown

import "testing"

// TestFormatMarkdown_RecoversFromRendererPanic pins the defensive recover in
// formatOnce: the goldmark-markdown renderer panics (nil deref) on certain
// inputs such as a bare link-reference-definition. FormatMarkdown must never
// crash the process — it returns the input unchanged instead.
func TestFormatMarkdown_RecoversFromRendererPanic(t *testing.T) {
	// Reproduces github.com/teekennedy/goldmark-markdown Renderer.Render nil deref.
	panicInputs := []string{
		"[00000000000000000000000000000000000000000000000000000000000000000000000000000]:0",
		"[x]:0",
	}
	for _, in := range panicInputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("FormatMarkdown panicked on %q: %v", in, r)
				}
			}()
			_ = FormatMarkdown(in) // must not panic; idempotency is separately fuzzed
		}()
	}
}
