// Package comments holds user commentary attached to entities: a remark about
// a property or a view section, stored alongside the graph rather than in it.
//
// # Why this is not in the graph
//
// A comment is a remark *about* an entity, not a fact *in* the domain model the
// operator declared in schema.yaml. Modeling one as an entity would mean
// fabricating a type the operator never wrote, and would drag commentary
// through every mechanism the graph owns: the audit log, version capture, the
// search index, `analyze_*`, and `/_schema`. None of those are wanted for a
// note someone left on a field.
//
// So this is a separate service with its own backends, deliberately outside
// store.Store and entitymanager — the same call [internal/userstate] makes for
// snoozes, and for the same reason.
//
// # Authorship is stamped, never supplied
//
// [Comment.Author] and [Comment.ID] are written by the server from the request
// principal; a client-supplied value for either is ignored. This is the reason
// the service owns its own write path rather than routing through the graph:
// none of the in-graph seams can stamp a trustworthy identity (automation
// template vars resolve to the *git config* user, computed properties cannot
// see the principal, and the elevated Lua write handle deliberately exposes no
// entity write). A forgeable author on a comment is a forgeable attribution of
// what someone said.
//
// # Ordering and identity are part of the contract
//
// [Store.List] returns comments oldest-first, ties broken by ID, so a thread
// renders identically on every backend. Implementations must not return
// storage order. [commentstest.RunAll] pins this, along with the concurrency
// and re-key behavior below — a contract asserted in one place beats three
// backends that each behave slightly differently.
//
// # Lifecycle
//
// Comments are keyed by target entity ID, so the service tracks the graph:
// [Service.EntityRenamed] re-keys a target's comments and
// [Service.EntityDelete] removes them. Rename emits exactly one store callback
// (never delete+put), so a service that ignored it would silently strand every
// comment on a renamed entity.
package comments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Sentinel errors. Callers map these to transport-level responses; the HTTP
// layer turns [ErrNotFound] into a 404 and the validation errors into 400s.
var (
	// ErrNotFound reports that no comment with the given ID exists on the
	// target. Returned by Update and Delete.
	ErrNotFound = errors.New("comments: not found")

	// ErrEmptyBody reports a comment whose body is empty or whitespace-only.
	ErrEmptyBody = errors.New("comments: body must not be empty")

	// ErrBodyTooLong reports a body over [MaxBodyBytes].
	ErrBodyTooLong = errors.New("comments: body too long")

	// ErrBodyControlChars reports a body containing control characters other
	// than newline and tab. The file backend serializes to YAML, where a NUL
	// is at best lossy and at worst breaks the document.
	ErrBodyControlChars = errors.New("comments: body contains control characters")

	// ErrTooManyComments reports that the target is at [MaxPerTarget].
	ErrTooManyComments = errors.New("comments: target has too many comments")

	// ErrUnknownAuthor reports an attempt to write with no resolved principal.
	// Refused rather than stored as a placeholder: an "unknown" author makes
	// every *-own permission check meaningless, since no one can prove
	// ownership of a comment nobody is recorded as having written.
	ErrUnknownAuthor = errors.New("comments: cannot attribute comment to an unknown principal")

	// ErrInvalidAnchor reports a structurally invalid anchor (unknown kind, or
	// an empty ref). Note this is NOT the "ref names nothing" case: an anchor
	// pointing at a property that has since been removed is a soft condition
	// per DEC-HWZHA and surfaces as a warning on read, never an error.
	ErrInvalidAnchor = errors.New("comments: invalid anchor")
)

// Limits on a single comment and on one target's thread. Both exist to bound
// the file backend, which holds a target's whole thread in one document that is
// read in full on every List.
const (
	// MaxBodyBytes caps a single comment body.
	MaxBodyBytes = 16 * 1024

	// MaxPerTarget caps how many comments one entity may carry.
	MaxPerTarget = 500
)

// AnchorKind identifies what part of an entity a comment is attached to.
//
// The set is deliberately open to extension: stage 2 adds a text-range kind
// carrying a quote and surrounding context, which is why [Anchor] is a struct
// with a kind discriminator rather than a bare property name. Adding a kind
// must not require migrating stored comments.
type AnchorKind string

const (
	// AnchorProperty attaches a comment to a named property. Ref is the
	// property name.
	AnchorProperty AnchorKind = "property"

	// AnchorSection attaches a comment to a view section. Ref is the
	// section's slug id, which is derived from the operator-authored view
	// heading rather than from user content — so it survives edits to the
	// entity body.
	AnchorSection AnchorKind = "section"
)

// Anchor locates a comment within its target entity.
//
// Both current kinds anchor by NAME (a property name, a section slug), never by
// offset, which is what makes them immune to edits of the entity body. That is
// the whole reason stage 1 ships these two kinds and defers text ranges.
type Anchor struct {
	Kind AnchorKind `json:"kind" yaml:"kind"`
	Ref  string     `json:"ref"  yaml:"ref"`
}

// Validate reports whether the anchor is structurally usable.
//
// It checks shape only. Whether Ref names a property or section that currently
// exists is deliberately NOT checked: an entity may be edited, or hand-edited
// outside rela, so a ref that resolves to nothing is an expected state that
// surfaces as a warning on read (DEC-HWZHA).
func (a Anchor) Validate() error {
	switch a.Kind {
	case AnchorProperty, AnchorSection:
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidAnchor, a.Kind)
	}
	if strings.TrimSpace(a.Ref) == "" {
		return fmt.Errorf("%w: ref must not be empty", ErrInvalidAnchor)
	}
	return nil
}

// Comment is one remark attached to one entity.
//
// ID, Author and CreatedAt are server-written on Add; a value supplied by a
// client for any of them is discarded (see the package doc). Body is markdown,
// stored verbatim and rendered — sanitized — by the caller.
type Comment struct {
	ID        string    `json:"id"         yaml:"id"`
	Author    string    `json:"author"     yaml:"author"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitzero" yaml:"updated_at,omitempty"`
	Anchor    Anchor    `json:"anchor"     yaml:"anchor"`
	Body      string    `json:"body"       yaml:"body"`
	Resolved  bool      `json:"resolved"   yaml:"resolved"`
}

// Target identifies the entity a thread of comments belongs to.
//
// Type is carried alongside ID because the HTTP surface is addressed by
// (type, id) and the read gate needs both; it is not part of the storage key,
// since entity IDs are unique across types.
type Target struct {
	Type string
	ID   string
}

// Store persists comments per target entity.
//
// Every implementation must pass [commentstest.RunAll], which pins the parts of
// this contract that are easy to get subtly different: List's ordering, the
// server-minted ID, and that concurrent Adds to one target both survive.
//
// Nil: no method returns a nil error with a nil result; List returns an empty
// slice rather than nil when a target has no comments.
type Store interface {
	// List returns the target's comments, oldest first, ties broken by ID.
	// A target with no comments yields an empty slice, not an error.
	List(ctx context.Context, target Target) ([]Comment, error)

	// Add appends c to the target's thread. The caller has already set ID,
	// Author and CreatedAt; implementations persist them as given rather
	// than minting their own, so the values in an audit trail and the values
	// stored agree.
	Add(ctx context.Context, target Target, c Comment) error

	// Update replaces the body and resolved flag of the comment with the
	// given ID. Returns [ErrNotFound] if it does not exist. Author,
	// CreatedAt and Anchor are immutable — editing a comment must not let it
	// change who said it or what it was about.
	Update(ctx context.Context, target Target, id string, body string, resolved bool) error

	// Delete removes one comment. Returns [ErrNotFound] if absent.
	Delete(ctx context.Context, target Target, id string) error

	// DeleteTarget removes every comment on a target. Deleting a target with
	// no comments is not an error.
	DeleteTarget(ctx context.Context, target Target) error

	// Rename re-keys a target's comments from oldID to newID, preserving
	// order. Renaming a target with no comments is not an error.
	Rename(ctx context.Context, oldID, newID string) error
}

// ValidateBody applies the size and character rules to a comment body.
//
// Allowlist-shaped: newline and tab are the only control characters permitted,
// because a body is prose. Everything else in the C0 range (and DEL) is
// refused rather than escaped, since the file backend round-trips through YAML
// where those bytes are lossy.
func ValidateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return ErrEmptyBody
	}
	if len(body) > MaxBodyBytes {
		return fmt.Errorf("%w: %d bytes exceeds the %d-byte limit", ErrBodyTooLong, len(body), MaxBodyBytes)
	}
	if !utf8.ValidString(body) {
		return fmt.Errorf("%w: body is not valid UTF-8", ErrBodyControlChars)
	}
	for _, r := range body {
		if r == '\n' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: U+%04X", ErrBodyControlChars, r)
		}
	}
	return nil
}
