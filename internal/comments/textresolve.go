package comments

import (
	"github.com/vloothuis/textanchor"
	"github.com/vloothuis/textanchor/quotefind"
)

// Confidence bands for a resolved text anchor.
//
// Three tiers rather than the library's binary resolved/orphaned, because the
// middle band is the one a reader needs warning about: a highlight rendered at
// 0.55 confidence is a guess, and presenting it identically to an exact match
// would quietly attach a remark to text nobody wrote it about.
const (
	// ConfidenceExact and above renders as a normal highlight.
	ConfidenceExact = 0.80

	// ConfidenceUncertain and above renders highlighted but flagged as moved;
	// below it the anchor is treated as detached. Matches the library's own
	// MinConfidence floor, so anything it resolves at all lands in a band.
	ConfidenceUncertain = 0.50
)

// TextMatch is the outcome of locating a text anchor in a body.
//
// Start/End are byte offsets into the body that was searched. They are NOT
// derivable from the quote's length: the resolver absorbs interior whitespace
// runs, so a range may legitimately be longer than Quote (a quote written with
// a space where the stored body now has a newline). Always slice with
// Start/End — never Start+len(Quote).
type TextMatch struct {
	Start      int
	End        int
	Confidence float64
	// Uncertain marks the middle band: located, but far enough from an exact
	// match that the UI should say so.
	Uncertain bool
	// Detached means the quote could not be found. Start/End are meaningless.
	Detached bool
	// Reason carries the resolver's explanation when Detached.
	Reason string
}

// ResolveText locates a text anchor within body.
//
// Nil: a nil descriptor resolves as detached rather than panicking — a stored
// comment with a text kind and no descriptor is corrupt, not a crash.
//
// The body is passed as stored. No normalisation happens here: textanchor
// v0.2.0 matches on a whitespace-collapsed form internally and maps its result
// back to original coordinates, so a quote spanning fsstore's 80-column reflow
// resolves without the caller flattening anything. (Before v0.2.0 that case
// hard-orphaned, which is why this function does not exist for v0.1.0.)
func ResolveText(body string, a *TextAnchor) TextMatch {
	if a == nil {
		return TextMatch{Detached: true, Reason: "missing text descriptor"}
	}

	res := textanchor.Resolve(body, textanchor.Anchor{
		Quote:              a.Quote,
		Prefix:             a.Prefix,
		Suffix:             a.Suffix,
		ContainingSentence: a.ContainingSentence,
		HeadingContext:     a.HeadingContext,
		ParagraphIndex:     a.ParagraphIndex,
	}, nil)

	if res.Orphaned || res.Range == nil {
		reason := res.OrphanReason
		if reason == "" {
			reason = "quote not found"
		}
		return TextMatch{Detached: true, Confidence: res.Confidence, Reason: reason}
	}

	// Defensive: a range outside the body would panic the caller's slice. The
	// resolver returns original-document coordinates, so this should never
	// fire — but a bad range must degrade to detached, not crash a read path.
	if res.Range.Start < 0 || res.Range.End > len(body) || res.Range.Start > res.Range.End {
		return TextMatch{Detached: true, Confidence: res.Confidence, Reason: "range out of bounds"}
	}

	return TextMatch{
		Start:      res.Range.Start,
		End:        res.Range.End,
		Confidence: res.Confidence,
		Uncertain:  res.Confidence < ConfidenceExact,
	}
}

// FindRenderedQuote locates a quote taken from RENDERED markdown within the
// markdown SOURCE.
//
// A browser selection yields display text: list markers, backticks and heading
// hashes are gone, and blocks are separated by plain newlines. A selection that
// crosses a bullet or a code span therefore never occurs verbatim in the
// source, so a plain strings.Index finds nothing — which is exactly what made
// "the selected text was not found" fire on those selections.
//
// quotefind walks the goldmark AST to build a rendered→source position map, so
// it matches what the user actually saw against what is actually stored.
//
// # prefix and suffix are what pick the RIGHT occurrence
//
// A short quote frequently occurs more than once ("eordend" appears inside both
// "Ongeordend" and "Geordend"). Without context this resolves to the FIRST
// occurrence, so a comment on the second silently anchors — and highlights —
// somewhere the user never selected. The caller must therefore pass the text
// surrounding the selection; empty context is only safe for a quote known to be
// unique.
//
// Returns ok=false when the quote cannot be located, which the caller reports
// as a 400 rather than storing an anchor that could never resolve.
func FindRenderedQuote(body, quote, prefix, suffix string) (start, end int, ok bool) {
	return quotefind.FindWithContext(body, quote, prefix, suffix)
}

// NewTextAnchor builds a text anchor for the selection body[start:end].
//
// Offsets are into the body AS STORED, so a caller working from rendered text
// must map back to source coordinates first — see [FindRenderedQuote].
func NewTextAnchor(body string, start, end int) (*TextAnchor, error) {
	a, err := textanchor.New(body, start, end, nil)
	if err != nil {
		return nil, err
	}
	return &TextAnchor{
		Quote:              a.Quote,
		Prefix:             a.Prefix,
		Suffix:             a.Suffix,
		ContainingSentence: a.ContainingSentence,
		HeadingContext:     a.HeadingContext,
		ParagraphIndex:     a.ParagraphIndex,
	}, nil
}
