package comments_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
)

const body = "Renaming an entity leaves the old id in the search index until a restart, " +
	"which is confusing because the store itself is already correct at that point.\n"

// anchorFor builds a text anchor over the first occurrence of quote in body.
func anchorFor(t *testing.T, doc, quote string) *comments.TextAnchor {
	t.Helper()
	start := strings.Index(doc, quote)
	require.GreaterOrEqual(t, start, 0, "quote not present in the fixture")
	a, err := comments.NewTextAnchor(doc, start, start+len(quote))
	require.NoError(t, err)
	return a
}

func TestResolveText_ExactMatch(t *testing.T) {
	quote := "the old id in the search index"
	got := comments.ResolveText(body, anchorFor(t, body, quote))

	require.False(t, got.Detached)
	require.False(t, got.Uncertain)
	require.Equal(t, quote, body[got.Start:got.End])
	require.GreaterOrEqual(t, got.Confidence, comments.ConfidenceExact)
}

// TestResolveText_SurvivesFormatMarkdownReflow is the reason stage 2 needs
// textanchor v0.2.0 at all.
//
// fsstore reflows every body to 80 columns on write, so a quote spanning a wrap
// point is split by a newline in the stored text. Against v0.1.0 that
// hard-orphaned at 0.00 confidence — an anchor breaking against its own body
// the moment the file was saved, with no user edit involved.
func TestResolveText_SurvivesFormatMarkdownReflow(t *testing.T) {
	reflowed := markdown.FormatMarkdown(body)
	require.NotEqual(t, body, reflowed, "fixture must actually be reflowed, or this proves nothing")

	quote := "until a restart, which is confusing"
	require.NotContains(t, reflowed, quote, "quote must straddle the wrap, or this proves nothing")

	got := comments.ResolveText(reflowed, anchorFor(t, body, quote))

	require.False(t, got.Detached, "quote spanning the reflow boundary must still resolve")
	require.GreaterOrEqual(t, got.Confidence, comments.ConfidenceExact)
	// The located range spans the inserted newline, so it is NOT the quote
	// verbatim — which is exactly why callers must slice with Start/End.
	require.Contains(t, reflowed[got.Start:got.End], "until a restart")
	require.Contains(t, reflowed[got.Start:got.End], "is confusing")
}

// TestResolveText_SliceWithRangeNotQuote pins the slicing contract.
//
// The resolved text is NOT the stored quote — here the wrap put a newline where
// the quote has a space — so a caller must slice with Start/End. Locating the
// span by searching for Quote instead would fail outright, which is the mistake
// this guards against. (Byte LENGTHS happen to agree in this fixture, since a
// newline replaces a space one-for-one; they need not in general, so do not
// rewrite this as a length assertion.)
func TestResolveText_SliceWithRangeNotQuote(t *testing.T) {
	reflowed := markdown.FormatMarkdown(body)
	quote := "until a restart, which is confusing"
	a := anchorFor(t, body, quote)

	got := comments.ResolveText(reflowed, a)
	require.False(t, got.Detached)

	located := reflowed[got.Start:got.End]
	require.NotEqual(t, quote, located, "the located text differs from the stored quote")
	require.Equal(t, -1, strings.Index(reflowed, quote),
		"searching for the quote would find nothing — only Start/End locate it")
}

func TestResolveText_DetachedWhenQuoteRemoved(t *testing.T) {
	a := anchorFor(t, body, "the old id in the search index")

	got := comments.ResolveText("Something else entirely, sharing no wording with the original.\n", a)

	require.True(t, got.Detached, "a quote edited away must detach, never match loosely")
	require.NotEmpty(t, got.Reason)
}

func TestResolveText_NilDescriptorDetaches(t *testing.T) {
	got := comments.ResolveText(body, nil)

	require.True(t, got.Detached)
	require.Equal(t, "missing text descriptor", got.Reason)
}

func TestResolveText_UnicodeQuote(t *testing.T) {
	doc := "Le café serveert bijzonder góéde köffie — élke ochtend opnieuw.\n"
	quote := "bijzonder góéde köffie"

	got := comments.ResolveText(doc, anchorFor(t, doc, quote))

	require.False(t, got.Detached)
	// Byte offsets over multi-byte runes must still slice to the exact quote.
	require.Equal(t, quote, doc[got.Start:got.End])
}

func TestAnchorValidate_Text(t *testing.T) {
	tests := []struct {
		name    string
		anchor  comments.Anchor
		wantErr bool
	}{
		{
			name:   "valid text anchor",
			anchor: comments.Anchor{Kind: comments.AnchorText, Text: &comments.TextAnchor{Quote: "a long enough quote"}},
		},
		{
			name:    "missing descriptor",
			anchor:  comments.Anchor{Kind: comments.AnchorText},
			wantErr: true,
		},
		{
			name:    "quote too short to locate",
			anchor:  comments.Anchor{Kind: comments.AnchorText, Text: &comments.TextAnchor{Quote: "ab"}},
			wantErr: true,
		},
		{
			name:    "quote over the byte cap",
			anchor:  comments.Anchor{Kind: comments.AnchorText, Text: &comments.TextAnchor{Quote: strings.Repeat("x", comments.MaxQuoteBytes+1)}},
			wantErr: true,
		},
		{
			// A text anchor carries no Ref, so requiring one would reject
			// every valid text comment.
			name:   "ref is not required for a text anchor",
			anchor: comments.Anchor{Kind: comments.AnchorText, Text: &comments.TextAnchor{Quote: "another valid quote"}},
		},
		{
			// Regression: the property/section kinds must keep requiring Ref.
			name:    "property still requires a ref",
			anchor:  comments.Anchor{Kind: comments.AnchorProperty},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.anchor.Validate()
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, comments.ErrInvalidAnchor)
				return
			}
			require.NoError(t, err)
		})
	}
}
