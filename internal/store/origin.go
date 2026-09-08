package store

import "context"

// OriginKind classifies the MECHANISM that produced a write, for history.
//
// It answers "how did these bytes get here", which is orthogonal to
// [Attribution]'s "who did it" and to VersionOp's "what happened to this row".
// A copy is genuinely a create-or-update of the target row — the op stays
// create/update — and additionally has a mechanism and a SOURCE, which is what
// this records.
type OriginKind string

const (
	// OriginCopy marks a write produced by a declared copy definition
	// (entitymanager's copy kernel). Source names the face it was copied FROM.
	OriginCopy OriginKind = "copy"
)

// Origin is the provenance of a single write: the mechanism that produced it
// and, when the mechanism has one, the source it was produced from.
//
// # Absence is the signal for a hand edit
//
// A write with NO Origin is a direct edit — a human (or a tool acting for one)
// supplying the bytes itself. There is deliberately no OriginKind("manual"):
// inventing a label for the default state would make "unmarked" ambiguous
// between "hand edit" and "written before this field existed", and the real
// signal for a hand edit is already complete — the absent origin plus the
// [Attribution] naming the editor. See TKT-ZIRMGM's identical reasoning for
// why absent attribution stays NULL rather than becoming a literal "unknown".
//
// # Reaching the store
//
// Origin reaches the store the same single sanctioned way [Attribution] does:
// populated at the entitymanager write boundary and carried on ctx. A store
// implementation never derives it from anything else. It describes the MOST
// RECENT write to a row, so an ordinary edit of a previously-copied row clears
// it — which is what makes "this version was copied" mean the version, not the
// row's distant past.
type Origin struct {
	Kind OriginKind

	// Source is the entity id this write was produced from. Empty when the
	// mechanism has no distinct source (a same-entity copy still sets it —
	// the id is the same but the FACE differs, and the pair is the source).
	Source string

	// SourceFace is the DECLARED NAME of the source's content-state face —
	// the spelling an operator wrote in the copy definition, e.g. "draft" —
	// NOT the stored coordinate entity.Face carries.
	//
	// The distinction is load-bearing and cost a bug to learn. A type may
	// name one of its faces `bare_face`, and that face's STORED coordinate is
	// the empty string: the bare id addresses it. So the bare face is
	// UNNAMEABLE as a coordinate, and a copy declared `from: policy@draft` on
	// a type with `bare_face: draft` recorded an empty SourceFace and read
	// back as a bare "POL-4" — losing precisely the fact provenance exists to
	// state. Provenance is a label a human reads, not an address anything
	// dereferences, so it holds the name.
	//
	// Empty therefore means "no face name to report": the source type
	// declares no faces at all, or declares faces but no bare_face and the
	// write came from its unnamed default state. It does NOT mean "the
	// default face" — that face has a name whenever the operator gave it one.
	SourceFace string

	// SourceType is the source entity's type. Stored ONLY so a read-out path
	// can gate the source id: an entity id is a row-level secret (whether an
	// entity EXISTS is a genuine secret, per the read-path ACL rule), and
	// PermitsRead is keyed by (type, id) — without the type, a reader-side
	// gate could not be applied and history would name a cross-entity source
	// the reader may not know exists. It is not display information.
	SourceType string

	// Definition is the operator-declared name that produced the write (for
	// OriginCopy, the key in the metamodel's `copies:` map). It is
	// configuration, not data — see the "configuration is not a secret" rule
	// — so recording and displaying it is deliberate.
	Definition string
}

// IsZero reports whether o carries no provenance at all, i.e. the write was a
// direct edit.
func (o Origin) IsZero() bool {
	return o.Kind == "" && o.Source == "" && o.SourceFace == "" &&
		o.SourceType == "" && o.Definition == ""
}

type originKey struct{}

// WithOrigin returns a ctx carrying o as the provenance for store writes made
// under it. Called only at the entitymanager write boundary. A zero Origin must
// not be set: absent provenance must stay absent so backends persist NULL and a
// reader can tell a hand edit from a copy.
func WithOrigin(ctx context.Context, o Origin) context.Context {
	return context.WithValue(ctx, originKey{}, o)
}

// OriginFrom returns the Origin carried by ctx, or the zero Origin when none
// was set. It never fabricates a default.
func OriginFrom(ctx context.Context) Origin {
	o, _ := ctx.Value(originKey{}).(Origin)
	return o
}

// SourceLabel renders the source face as "ID@face", matching the spelling
// used in copy definitions and the copy audit summary. Empty when the origin
// names no source.
//
// It falls back to a bare "ID" only when [Origin.SourceFace] is empty, which
// per that field means there is no declared name to print — NOT that the
// source was "the default face". A named bare face still renders "POL-4@draft",
// because the writer records the declared name rather than the stored
// coordinate.
//
// This lives here rather than in each renderer so the CLI, the HTTP wire and
// any future reader agree on ONE spelling — a reader who has learned to read
// "POL-1@draft" in one place reads the same string in the others. That shared
// spelling is also why the bare-face fix belongs at the WRITE boundary: a
// read-side mapping would need a metamodel at every renderer, and this method
// (which has none, and must not import one) would stop being the single
// source of the spelling.
func (o Origin) SourceLabel() string {
	if o.Source == "" {
		return ""
	}
	if o.SourceFace == "" {
		return o.Source
	}
	return o.Source + "@" + o.SourceFace
}
