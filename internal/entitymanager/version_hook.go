package entitymanager

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// VersionRecorder captures a synchronous entity version at the write
// choke-point. Only rename and delete are recorded here: they carry
// information a later reconciliation sweep cannot reconstruct — a rename's
// old->new id link, and a delete's pre-delete state (the row is gone before any
// sweep runs). create/update are captured by the backend's periodic sweep.
//
// This is a consumer-side interface (declared where it is used, not next to the
// implementation): Manager depends on exactly the one method it calls. The
// postgres wiring supplies a pgstore-backed recorder; other backends supply a
// no-op (or leave it nil). Capture is best-effort — a recorder error must never
// fail the underlying write — so the method returns an error only for the hook
// to log, not to propagate.
type VersionRecorder interface {
	// RecordVersion persists one entity version. proj is the render-schema
	// projection JSON the snapshot was taken under and projHash its content
	// hash (from metamodel.RenderProjection().Hash()).
	RecordVersion(ctx context.Context, v VersionRecord) error
}

// VersionRecord is one entity version to capture: the snapshot state, the op,
// the rename predecessor id (rename only), attribution, and the render-schema
// projection the snapshot was taken under.
type VersionRecord struct {
	EntityID string
	// Face is the content state (face) this record captures; zero is the
	// default face (TKT-C1XUA8).
	Face          entity.Face
	Op            store.VersionOp
	PrevID        string // rename only: the entity's former id
	Type          string
	Content       string
	Properties    map[string]any
	SchemaHash    string
	Projection    []byte
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string
}

// recordEntityVersion builds a VersionRecord from the entity's current state
// and dispatches it to the recorder. No-op when no recorder is wired (fs/mem
// builds). Attribution is read from ctx exactly like audit — never from a
// caller-supplied field — so a version row cannot carry a forged principal.
// Failure is logged, never propagated: versioning must not block a write.
func (m *Manager) recordEntityVersion(ctx context.Context, op store.VersionOp, e *entity.Entity, prevID string) {
	if m.deps.VersionRecorder == nil {
		return
	}
	// PER-STATE (TKT-C1XUA8): the Step-1 skip is gone. entity_versions now
	// keys (entity_id, face, vseq), so a state capture gets its own
	// lineage instead of interleaving with the family's other faces, and
	// the face travels on the record below.
	proj := m.deps.Meta.RenderProjection()
	projJSON, err := proj.JSON()
	if err != nil {
		// Never fail the write over versioning — log and skip this capture.
		slog.Error("version.projection_marshal_failed", "op", op, "id", e.ID, "error", err)
		return
	}
	p := principal.From(ctx)
	rec := VersionRecord{
		EntityID:      e.ID,
		Face:          e.Face,
		Op:            op,
		PrevID:        prevID,
		Type:          e.Type,
		Content:       e.Content,
		Properties:    e.Properties,
		SchemaHash:    proj.Hash(),
		Projection:    projJSON,
		PrincipalUser: p.User,
		PrincipalTool: p.Tool,
		TriggeredBy:   audit.TriggeredByFrom(ctx),
	}
	if err := m.deps.VersionRecorder.RecordVersion(ctx, rec); err != nil {
		slog.Error("version.record_failed", "op", op, "id", e.ID, "error", err)
	}
}

// RelationVersionRecorder captures a synchronous relation version at the write
// choke-point. Like VersionRecorder it records only rename/delete — the ops the
// backend's reconciliation sweep cannot reconstruct (a delete's pre-delete state,
// since the row is gone before any sweep runs; a rename's pre-rename endpoints).
// create/update are captured by the sweep. A consumer-side interface: Manager
// depends on exactly the one method it calls; the postgres wiring supplies a
// pgstore-backed recorder, other backends a nil.
type RelationVersionRecorder interface {
	// RecordRelationVersion persists one relation version. Best-effort — an
	// error is returned only for the hook to log, never to fail the write.
	RecordRelationVersion(ctx context.Context, v RelationVersionRecord) error
}

// RelationVersionRecord is one relation version to capture: the snapshot state,
// the op, the pre-rename endpoints (rename only), attribution, and the
// render-schema projection the snapshot was taken under.
type RelationVersionRecord struct {
	From          string
	Type          string
	To            string
	Op            store.VersionOp
	PrevFrom      string // rename only: the relation's former from endpoint
	PrevTo        string // rename only: the relation's former to endpoint
	Content       string
	Properties    map[string]any
	SchemaHash    string
	Projection    []byte
	PrincipalUser string
	PrincipalTool string
	TriggeredBy   string
}

// recordRelationVersion builds a RelationVersionRecord from a relation's state
// and dispatches it. No-op when no recorder is wired. Attribution is read from
// ctx (never a caller-supplied field). triggeredBy overrides the ctx-derived
// value when non-empty (a cascade delete attributes each relation to the entity
// delete that triggered it). Failure is logged, never propagated.
func (m *Manager) recordRelationVersion(
	ctx context.Context, op store.VersionOp, r *entity.Relation, prevFrom, prevTo, triggeredBy string,
) {
	if m.deps.RelationVersionRecorder == nil {
		return
	}
	// STILL SKIPPED, and now for a narrower reason (TKT-C1XUA8).
	//
	// TKT-C1XUA8 gave state-tailed edges their own history — the SWEEP
	// captures them, because it reads rel_record_id straight off the row.
	// This SYNCHRONOUS path cannot: the record it builds carries the
	// (from, type, to) TRIPLE, and the store resolves that to a lineage via
	// recordIDForKey, which answers with the DEFAULT tail by design (a key
	// with no face names the default face). Capturing a state-tailed edge
	// here would therefore file it under the default tail's lineage — the
	// interleaving the whole per-state design exists to prevent.
	//
	// Lifting this needs the record to carry a rel_record_id (or a tail) so
	// the store can address the right lineage. Until then the sync path —
	// rename stitch and pre-delete capture — is default-tail only, and a
	// state edge's delete is captured by the entity cascade instead.
	if !r.FromFace.IsDefault() {
		return
	}
	proj := m.deps.Meta.RenderProjection()
	projJSON, err := proj.JSON()
	if err != nil {
		slog.Error("relation_version.projection_marshal_failed",
			"op", op, "from", r.From, "type", r.Type, "to", r.To, "error", err)
		return
	}
	tb := audit.TriggeredByFrom(ctx)
	if triggeredBy != "" {
		tb = triggeredBy
	}
	p := principal.From(ctx)
	rec := RelationVersionRecord{
		From:          r.From,
		Type:          r.Type,
		To:            r.To,
		Op:            op,
		PrevFrom:      prevFrom,
		PrevTo:        prevTo,
		Content:       r.Content,
		Properties:    r.Properties,
		SchemaHash:    proj.Hash(),
		Projection:    projJSON,
		PrincipalUser: p.User,
		PrincipalTool: p.Tool,
		TriggeredBy:   tb,
	}
	if err := m.deps.RelationVersionRecorder.RecordRelationVersion(ctx, rec); err != nil {
		slog.Error("relation_version.record_failed",
			"op", op, "from", r.From, "type", r.Type, "to", r.To, "error", err)
	}
}
