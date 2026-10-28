package comments

import (
	"errors"
	"fmt"
	"strings"
)

// BuildTextAnchor derives a text anchor's descriptors from the entity's own
// body, given the quote the client selected.
//
// The client supplies only the quote; everything else (prefix, suffix,
// sentence, heading, paragraph index) is computed here. That asymmetry is
// deliberate and is the security-relevant part: descriptors sourced from the
// request could describe context that does not exist in the entity, so a later
// resolve would land on text the commenter never selected.
//
// WHICH occurrence was meant is decided by the surrounding rendered text the
// client sends alongside the quote. That context is untrusted in the sense that
// it only selects among real occurrences — it can never introduce a location,
// because the quote must still be found in the body. Without it a repeated
// quote resolves to the first occurrence, which is how a comment on "Geordend"
// ended up highlighting "Ongeordend".
func BuildTextAnchor(content, quote, prefix, suffix string) (*TextAnchor, error) {
	quote = strings.TrimSpace(quote)
	if len([]rune(quote)) < MinQuoteRunes {
		return nil, fmt.Errorf("selected text must be at least %d characters", MinQuoteRunes)
	}
	if len(quote) > MaxQuoteBytes {
		return nil, fmt.Errorf("selected text exceeds %d bytes", MaxQuoteBytes)
	}

	// The quote came from RENDERED markdown, so it is display text: no list
	// markers, no backticks, blocks joined by newlines. Matching it against the
	// source with a plain substring search fails for any selection crossing a
	// bullet or a code span — FindRenderedQuote maps through the AST instead.
	start, end, ok := FindRenderedQuote(content, quote, prefix, suffix)
	if !ok {
		// Either a stale tab (the body changed between render and submit) or a
		// selection that genuinely spans nothing locatable.
		return nil, errors.New("the selected text was not found in the current body")
	}

	return NewTextAnchor(content, start, end)
}
