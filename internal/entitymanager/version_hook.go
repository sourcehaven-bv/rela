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
	EntityID      string
	Op            store.VersionOp
	PrevID        string // rename only: the entity's former id
	Type          string
	Content       string
	Properties    map[string]interface{}
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
