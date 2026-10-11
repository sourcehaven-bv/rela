package comments

import (
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// The types below are the transport-neutral shape of the comment operations
// (TKT-QIQO1W). The operations themselves stay with the data-entry handler,
// which owns the read gates, the ACL and the entity write an accept performs;
// these types are what it hands to a second transport (the MCP tools) so that
// one implementation of the gates serves both.

// View is one comment as the requesting principal sees it.
//
// It is a projection, not the stored [Comment]: the anchor carries only what
// the HTTP list serves (kind, ref, quote, replacement), never the context the
// store captured around a quote, which describes a body the caller may no
// longer be able to read.
//
// The flags are computed per read against the live entity and the caller's
// grants, never stored: Detached, Editable and Deletable as on the HTTP list,
// Acceptable as there too (an open suggestion that still locates; it does not
// include the entity update right, which the accept re-checks).
type View struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	// CreatedAt is UTC, to the second, as on the HTTP list.
	CreatedAt time.Time  `json:"created_at"`
	Anchor    AnchorView `json:"anchor"`
	Body      string     `json:"body"`
	Resolved  bool       `json:"resolved"`

	Detached   bool `json:"detached,omitempty"`
	Editable   bool `json:"editable"`
	Deletable  bool `json:"deletable"`
	Acceptable bool `json:"acceptable,omitempty"`
}

// AnchorView is the part of an [Anchor] a reader is shown.
type AnchorView struct {
	Kind        AnchorKind `json:"kind"`
	Ref         string     `json:"ref,omitempty"`
	Quote       string     `json:"quote,omitempty"`
	Replacement *string    `json:"replacement,omitempty"`
}

// NewView projects c for a reader. The caller sets the flags.
func NewView(c Comment) View {
	return View{
		ID:        c.ID,
		Author:    c.Author,
		CreatedAt: c.CreatedAt.UTC().Truncate(time.Second),
		Anchor:    c.Anchor.View(),
		Body:      c.Body,
		Resolved:  c.Resolved,
	}
}

// View is the part of a shown to a reader.
func (a Anchor) View() AnchorView {
	out := AnchorView{Kind: a.Kind, Ref: a.Ref, Replacement: a.Replacement}
	if a.Text != nil {
		out.Quote = a.Text.Quote
	}
	return out
}

// AddInput is a request to add a comment.
//
// For a text anchor the caller supplies only the quote and its surrounding
// text; the stored descriptors are derived from the entity body, as on the
// HTTP route, so a caller cannot persist context the entity does not have.
type AddInput struct {
	Kind        AnchorKind
	Ref         string
	Quote       string
	QuotePrefix string
	QuoteSuffix string
	// Replacement suggests substitute text for the quote (text anchors only).
	// Nil is no suggestion; "" suggests deleting the quote.
	Replacement *string
	Body        string
}

// Accepted is the result of applying a comment's suggested replacement.
type Accepted struct {
	// Content is the entity body after the write.
	Content string `json:"content"`
	// Warnings are the soft validation findings of the write.
	Warnings []entity.Warning `json:"warnings,omitempty"`
}

// OpError is a refused comment operation, in the vocabulary of the HTTP API:
// Code is the error type's last segment (`not_found`, `forbidden`,
// `face_required`, …), and Title and Detail are the texts the HTTP route
// answers with. A second transport renders it instead of re-deriving the
// wording, so the two surfaces cannot drift.
type OpError struct {
	Code   string
	Title  string
	Detail string
	// Faces lists the addressable faces for `face_required`.
	Faces []string
}

func (e *OpError) Error() string {
	var b strings.Builder
	b.WriteString(e.Title)
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	if len(e.Faces) > 0 {
		b.WriteString(" (faces: ")
		b.WriteString(strings.Join(e.Faces, ", "))
		b.WriteString(")")
	}
	if e.Code != "" {
		b.WriteString(" [")
		b.WriteString(e.Code)
		b.WriteString("]")
	}
	return b.String()
}

// LiveProperties is the set of names a property anchor counts as present in:
// the type's declared properties, set or not, since a comment on an empty
// field is about a field that exists; and any property set on the entity,
// since hand-edited frontmatter can carry names the schema lacks.
func LiveProperties[D, S any](declared map[string]D, set map[string]S) map[string]bool {
	names := make(map[string]bool, len(declared)+len(set))
	for name := range declared {
		names[name] = true
	}
	for name := range set {
		names[name] = true
	}
	return names
}

// Detached reports whether a no longer names anything on an entity: a
// property anchor whose name is not in properties (see [LiveProperties]), or
// a text anchor whose quote body cannot locate. A section anchor is never
// detached here, since it names a view heading rather than part of the
// entity.
func Detached(a Anchor, properties map[string]bool, body *Body) bool {
	switch a.Kind {
	case AnchorProperty:
		return !properties[a.Ref]
	case AnchorText:
		return body.locate(a.Text).Detached
	default:
		return false
	}
}
