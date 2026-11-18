package visibility

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ChangeOps are the audit ops a per-entity change list shows: the writes that
// changed an entity or one of its relations. Every other op stays out,
// because it records a read, a refusal, an operator action or a record that
// is not about the entity's content. TestChangeOpsClassifyEveryOp fails for
// an op that is in neither this list nor its exclusions.
var ChangeOps = []string{
	audit.OpCreateEntity,
	audit.OpUpdateEntity,
	audit.OpDeleteEntity,
	audit.OpRenameEntity,
	audit.OpConvertEntity,
	audit.OpRestoreEntity,
	audit.OpPurgeDeletedEntity,
	audit.OpCopyState,
	audit.OpAttachRender,
	audit.OpCreateRelation,
	audit.OpUpdateRelation,
	audit.OpDeleteRelation,
}

// AuditEntries is the ungated audit source [EntityChanges] reads: one face
// of an entity's records, newest first, paged by seq. sqlaudit.Log
// implements it.
type AuditEntries interface {
	ForEntity(ctx context.Context, q audit.SubjectQuery, before int64, limit int) ([]audit.Entry, error)
}

// ReadableIDs answers which of ids the principal may read some face of, with
// each one's type. [Resolver.ReadableTypes] implements it.
type ReadableIDs interface {
	ReadableTypes(ctx context.Context, ids []string) (map[string]string, error)
}

// FieldVisibility answers whether the principal sees a property on every
// row of a type. A change list names a changed property only when it does,
// because a summary that names a hidden property tells when its value
// changed.
type FieldVisibility interface {
	FieldVisibleOnEveryRow(ctx context.Context, entityType, property string) bool
}

// ChangeRecord is one entry of an entity's change list as a reader may see
// it. It is built field by field from an audit record, never copied, so a
// field the record gains later stays off this view until it is added here.
//
// Summary is kept only in a shape that names no entity (property, type and
// face names are configuration, not data); Trigger is the kind of what
// caused the write, with an automation or schedule name but no entity id.
type ChangeRecord struct {
	Seq     int64          `json:"seq"`
	Time    time.Time      `json:"time"`
	Op      string         `json:"op"`
	Subject *audit.Subject `json:"subject,omitempty"`
	Before  *audit.Subject `json:"before,omitempty"`
	After   *audit.Subject `json:"after,omitempty"`
	User    string         `json:"user,omitempty"`
	Tool    string         `json:"tool,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Trigger string         `json:"trigger,omitempty"`
}

// ChangesQuery names the entity face whose changes [EntityChanges] reads.
type ChangesQuery struct {
	Ref  entity.Ref
	Type string
	// Before is the page cursor: the seq of the last record of the previous
	// page, or 0 for the newest page.
	Before int64
	Limit  int
}

// changeScanRounds caps the source reads one page makes while records it
// may not show fill the reads. Past the cap the page is served short, with
// a cursor, so a page costs at most this many source reads and one
// readability batch per read.
const changeScanRounds = 4

// EntityChanges returns one page of the changes of q.Ref, newest first, and
// the cursor of the next page (0 when there is none).
//
// The caller has already decided that the principal may see q.Ref's
// history; this function decides which records of it the principal may see.
// A record is shown only when every other entity it names (a relation's far
// end, a rename's other id) is readable to the principal now, by one
// [ReadableIDs] batch per source read. An entity record must be of q.Type,
// so a record of another type that held the id is never shown. A record of
// a hidden or deleted far end is left out entirely, as the live relation
// list leaves out a hidden neighbour. So is an edge into q.Ref from one face
// of another entity, because readability is answered per id, not per face.
//
// Records left out do not shorten a page while the source has more: the
// page reads on, up to [changeScanRounds] reads.
func EntityChanges(
	ctx context.Context, src AuditEntries, readable ReadableIDs, fields FieldVisibility, q ChangesQuery,
) ([]ChangeRecord, int64, error) {
	eq := audit.SubjectQuery{ID: q.Ref.ID, Face: q.Ref.Face.String(), Ops: ChangeOps}
	var out []ChangeRecord
	before := q.Before
	for range changeScanRounds {
		entries, err := src.ForEntity(ctx, eq, before, q.Limit+1)
		if err != nil {
			return nil, 0, err
		}
		shown, err := showable(ctx, readable, q, entries)
		if err != nil {
			return nil, 0, err
		}
		for _, e := range shown {
			if len(out) == q.Limit {
				return out, out[len(out)-1].Seq, nil
			}
			out = append(out, changeRecord(ctx, fields, e))
		}
		if len(entries) <= q.Limit {
			return out, 0, nil
		}
		before = entries[len(entries)-1].Seq
	}
	// The scan stopped at its cap with more to read: continue after the
	// last record read, shown or not.
	return out, before, nil
}

// showable returns the entries of page the principal may see.
func showable(ctx context.Context, readable ReadableIDs, q ChangesQuery, page []audit.Entry) ([]audit.Entry, error) {
	var others []string
	for _, e := range page {
		others = append(others, otherIDs(e.Record, q.Ref.ID)...)
	}
	types := map[string]string{}
	if len(others) > 0 {
		var err error
		if types, err = readable.ReadableTypes(ctx, others); err != nil {
			return nil, err
		}
	}
	shown := page[:0:0]
	for _, e := range page {
		if !aboutEntity(e.Record, q) {
			continue
		}
		ok := true
		for _, id := range otherIDs(e.Record, q.Ref.ID) {
			if _, readable := types[id]; !readable {
				ok = false
				break
			}
		}
		if s := e.Record.Subject; s != nil && s.FromID != q.Ref.ID && s.FromFace != "" {
			// An edge into this entity from one face of another: the batch
			// says some face of its tail is readable, not that one.
			ok = false
		}
		if ok {
			shown = append(shown, e)
		}
	}
	return shown, nil
}

// aboutEntity reports whether rec is an entity or relation record that names
// q.Ref.ID, with every entity subject naming it of q.Type.
func aboutEntity(rec audit.Record, q ChangesQuery) bool {
	named := false
	for _, s := range []*audit.Subject{rec.Subject, rec.Before, rec.After} {
		if s == nil {
			continue
		}
		switch s.Kind {
		case "entity":
			if s.ID == q.Ref.ID {
				if s.Type != q.Type && !isConversionBefore(rec, s) {
					return false
				}
				named = true
			}
		case "relation":
			if s.FromID == q.Ref.ID || s.ToID == q.Ref.ID {
				named = true
			}
		default:
			return false
		}
	}
	return named
}

// isConversionBefore reports whether s is the Before of a conversion, which
// names the entity's earlier type.
func isConversionBefore(rec audit.Record, s *audit.Subject) bool {
	return rec.Op == audit.OpConvertEntity && s == rec.Before && rec.After != nil && rec.After.ID == s.ID
}

// otherIDs returns the ids rec names besides id.
func otherIDs(rec audit.Record, id string) []string {
	var out []string
	for _, s := range []*audit.Subject{rec.Subject, rec.Before, rec.After} {
		if s == nil {
			continue
		}
		for _, other := range []string{s.ID, s.FromID, s.ToID} {
			if other != "" && other != id {
				out = append(out, other)
			}
		}
	}
	return out
}

func changeRecord(ctx context.Context, fields FieldVisibility, e audit.Entry) ChangeRecord {
	r := e.Record
	summary := ChangeSummary(r.Summary)
	if props, found := strings.CutPrefix(summary, updatedPrefix); found {
		summary = visibleUpdate(ctx, fields, r.Subject, props)
	}
	return ChangeRecord{
		Seq:     e.Seq,
		Time:    r.Time,
		Op:      r.Op,
		Subject: copySubject(r.Subject),
		Before:  copySubject(r.Before),
		After:   copySubject(r.After),
		User:    r.Principal.User,
		Tool:    r.Principal.Tool,
		Summary: summary,
		Trigger: TriggerKind(r.TriggeredBy),
	}
}

func copySubject(s *audit.Subject) *audit.Subject {
	if s == nil {
		return nil
	}
	return &audit.Subject{
		Kind: s.Kind, Type: s.Type, ID: s.ID, Face: s.Face,
		RelationType: s.RelationType, FromID: s.FromID, FromFace: s.FromFace, ToID: s.ToID,
	}
}

// updatedPrefix starts the summary of an update that lists its properties.
const updatedPrefix = "updated: "

// visibleUpdate returns the summary of an update that changed props, naming
// only the properties the principal sees on every row of the subject's type.
// A relation's properties are never named: their visibility is judged per
// relation, which this list does not do.
func visibleUpdate(ctx context.Context, fields FieldVisibility, subject *audit.Subject, props string) string {
	if subject == nil || subject.Kind != "entity" {
		return "updated"
	}
	var shown []string
	for _, p := range strings.Split(props, ",") {
		if fields.FieldVisibleOnEveryRow(ctx, subject.Type, p) {
			shown = append(shown, p)
		}
	}
	if len(shown) == 0 {
		return "updated"
	}
	return updatedPrefix + strings.Join(shown, ",")
}

// name is a configuration name: a property, type, face, conversion or
// automation name. An entity id would also match, so a summary shape below
// puts a name only where the write path puts a configuration name.
const name = `[A-Za-z0-9_.-]+`

// summaryShapes are the summaries a change list keeps. Each names only
// configuration; one that may name an entity (a copy's source and target, a
// mention rewrite's ids) or count hidden rows (a cascade's relation count) is
// not here or has the count cut first.
var summaryShapes = regexp.MustCompile(`^(?:` + strings.Join([]string{
	`(?:created|deleted|restored|renamed|updated)(?: \(undoable\))?(?: face ` + name + `)?`,
	`updated: ` + name + `(?:,` + name + `)*`,
	`(?:renumbered|moved) ` + name,
	`(?:created|removed) by conversion to ` + name,
	`converted from ` + name + ` by ` + name + `(?: face ` + name + `)?`,
	`purged after soft delete`,
}, "|") + `)$`)

// countSuffix is the relation count a delete or purge summary ends with.
var countSuffix = regexp.MustCompile(` \((?:cascade: )?\d+ relations\)$`)

// ChangeSummary returns summary when it has a shape that names no entity,
// with any relation count cut, and "" otherwise.
func ChangeSummary(summary string) string {
	s := countSuffix.ReplaceAllString(summary, "")
	if summaryShapes.MatchString(s) {
		return s
	}
	return ""
}

// namedTriggers are the trigger kinds whose label after the colon is a
// configuration name, kept whole.
var namedTriggers = map[string]bool{"automation": true, "schedule": true, "renumber": true}

// TriggerKind returns the kind of a triggered_by label: the part before the
// first colon, or the whole label for an automation, schedule or renumber,
// whose rest is a configuration name. Other kinds carry entity ids
// (cascade:delete-entity:<id>, rename:<old>-><new>), which a change list
// does not gate.
func TriggerKind(label string) string {
	kind, rest, found := strings.Cut(label, ":")
	if !found {
		return label
	}
	if namedTriggers[kind] && !strings.Contains(rest, ":") {
		return label
	}
	return kind
}
