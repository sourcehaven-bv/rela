package datamigration

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/computed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// migrationTool is the attribution tool name stamped on every write a
// migration run makes (store.WithAttribution → last_edited_by_tool on pg).
const migrationTool = "data-migration"

// updateBatchSize bounds how many entity updates share one store.Tx. One
// giant transaction over the whole graph is forbidden (on pg it stalls every
// writer for the duration); per-entity transactions waste round-trips.
const updateBatchSize = 200

// VersionCapture is the slice of the versioning service the runner needs for
// synchronous pre-delete capture (amendment A1) — nil on fs/mem builds,
// where git is the recovery path.
type VersionCapture interface {
	WriteVersion(ctx context.Context, in store.VersionInput) error
	WriteRelationVersion(ctx context.Context, in store.RelationVersionInput) error
}

// Deps are the runner's collaborators. Store, Meta, State, Audit, ScriptFS
// and Lock are required; Versions is optional (pg only).
type Deps struct {
	Store    store.Store
	Meta     *metamodel.Metamodel
	State    state.KV
	Audit    audit.Audit
	ScriptFS fs.FS // project root, for `lua:` step scripts
	Versions VersionCapture
	// Lock serializes apply runs against every other migration/GC writer on
	// the same store (TKT-CPCBR7); build it with [LockFor]. Required even
	// though dry-runs never touch it: a destructive path must not lose its
	// serialization to a forgotten wiring line.
	Lock MigrationLock
}

// Runner executes a resolved migration plan against one store.
type Runner struct {
	deps     Deps
	computed *computed.Set
	now      func() time.Time
}

// NewRunner validates required collaborators up front.
func NewRunner(deps Deps) (*Runner, error) {
	switch {
	case deps.Store == nil:
		return nil, errors.New("datamigration: NewRunner: Store is required")
	case deps.Meta == nil:
		return nil, errors.New("datamigration: NewRunner: Meta is required")
	case deps.State == nil:
		return nil, errors.New("datamigration: NewRunner: State is required")
	case deps.Audit == nil:
		return nil, errors.New("datamigration: NewRunner: Audit is required")
	case deps.ScriptFS == nil:
		return nil, errors.New("datamigration: NewRunner: ScriptFS is required")
	case deps.Lock == nil:
		return nil, errors.New("datamigration: NewRunner: Lock is required (use LockFor)")
	}
	computedSet, err := computed.Compile(deps.Meta)
	if err != nil {
		return nil, fmt.Errorf("datamigration: NewRunner: compile computed properties: %w", err)
	}
	return &Runner{deps: deps, computed: computedSet, now: time.Now}, nil
}

// RunResult reports one Run invocation.
type RunResult struct {
	Applied bool // false = dry-run
	Files   []FileResult
	// ValidationBefore/After count entities failing property validation
	// against the LIVE metamodel before and after the run — the "will this
	// migration actually heal the data" signal. Only computed on apply and
	// dry-run alike (a read-only scan).
	ValidationBefore int
	ValidationAfter  int
}

// FileResult reports one migration file's execution.
type FileResult struct {
	Name  string
	From  string
	To    string
	Steps []StepResult
}

// Run executes the plan (from [Resolve]) in order. Dry-run (apply=false)
// counts affected records per step without writing. On apply, the marker
// advances after EACH file completes — a crash between files resumes at the
// right position — and one audit record is emitted per applied file.
//
// The whole run executes under a system-attributed context: audit and
// version rows attribute to the invoking operator with the data-migration
// tool, never to a guessed identity.
func (r *Runner) Run(ctx context.Context, plan []*File, apply bool) (*RunResult, error) {
	if apply {
		// The whole apply run holds the migration lock: marker advances and
		// bulk rewrites must not interleave with another runner, a GC apply,
		// or a gate adoption. Dry-runs are read-only and stay lock-free.
		release, err := r.deps.Lock.TryAcquire(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	p := principal.From(ctx)
	ctx = store.WithAttribution(ctx, store.Attribution{User: p.User, Tool: migrationTool})

	res := &RunResult{Applied: apply}
	res.ValidationBefore = len(schema.ValidateEntityProperties(ctx, r.deps.Store, r.deps.Meta))

	for _, f := range plan {
		x := &Exec{
			Store:    r.deps.Store,
			Apply:    apply,
			ScriptFS: r.deps.ScriptFS,
			Computed: r.computed,
			capture:  r.newCapturer(f.Name),
		}
		fr := FileResult{Name: f.Name, From: f.From, To: f.To}
		for _, step := range f.Steps {
			sr, err := step.Run(ctx, x)
			fr.Steps = append(fr.Steps, sr)
			if err != nil {
				res.Files = append(res.Files, fr)
				return res, fmt.Errorf("datamigration: %s: step %s (%s): %w", f.Name, sr.Kind, sr.Target, err)
			}
		}
		res.Files = append(res.Files, fr)

		if apply {
			if err := r.advanceMarker(ctx, f); err != nil {
				return res, err
			}
			r.auditFile(p, f, fr)
		}
	}

	res.ValidationAfter = len(schema.ValidateEntityProperties(ctx, r.deps.Store, r.deps.Meta))
	return res, nil
}

// advanceMarker moves the marker to the file's to-shape and appends the file
// to the applied list. Written only after every step of the file succeeded —
// the marker must never claim conformance the data doesn't have.
func (r *Runner) advanceMarker(ctx context.Context, f *File) error {
	marker, err := LoadMarker(ctx, r.deps.State)
	if err != nil {
		return err
	}
	var applied []string
	if marker != nil {
		applied = marker.Applied
	}
	applied = appendUnique(applied, f.Name)
	m, err := NewMarker(f.ToProjection, applied, r.now().UTC())
	if err != nil {
		return err
	}
	return SaveMarker(ctx, r.deps.State, m)
}

// auditFile emits the per-file audit record: names, hashes and counts —
// never content (history-purge convention).
func (r *Runner) auditFile(p principal.Principal, f *File, fr FileResult) {
	total := 0
	for _, s := range fr.Steps {
		total += s.Affected
	}
	r.deps.Audit.Record(audit.Record{
		Time:      r.now().UTC(),
		Op:        audit.OpDataMigration,
		Principal: p,
		Summary: fmt.Sprintf("data migration %s applied (%s → %s): %d steps, %d records changed",
			f.Name, short(f.From), short(f.To), len(fr.Steps), total),
	})
}

func appendUnique(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}

// ---- execution context shared by steps ----

// Exec is the per-file execution context handed to every step.
type Exec struct {
	Store    store.Store
	Apply    bool
	ScriptFS fs.FS
	Computed *computed.Set
	capture  *capturer
}

// forEachEntity runs fn over every entity of typ (collect-then-write, the
// normalize.go pattern: never mutate while iterating). Entities fn reports
// changed are written back in Tx batches; in dry-run they are only counted.
func (x *Exec) forEachEntity(
	ctx context.Context, typ string, fn func(e *entity.Entity) (bool, error), res *StepResult,
) error {
	q := store.EntityQuery{}
	if typ != "*" {
		q.Type = typ
	}
	var changed []*entity.Entity
	for e, err := range x.Store.ListEntities(ctx, q) {
		if err != nil {
			return err
		}
		did, err := fn(e)
		if err != nil {
			return fmt.Errorf("%s: %w", e.ID, err)
		}
		if did {
			changed = append(changed, e)
		}
	}
	res.Affected += len(changed)
	if !x.Apply {
		return nil
	}
	for start := 0; start < len(changed); start += updateBatchSize {
		batch := changed[start:min(start+updateBatchSize, len(changed))]
		err := x.Store.Tx(ctx, func(s store.Store) error {
			for _, e := range batch {
				if err := s.UpdateEntity(ctx, e); err != nil {
					return fmt.Errorf("%s: %w", e.ID, err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// captureEntityDelete records a synchronous pre-delete version (A1); no-op
// without a versioning service. Unlike the entitymanager's best-effort hook,
// a FAILED capture here is a hard error: these deletes are bulk and (on the
// GC path) unattended, so "delete anyway" would destroy the only copy with
// no history row. The caller aborts; the operator retries once versioning is
// healthy — the steps are idempotent.
func (x *Exec) captureEntityDelete(ctx context.Context, e *entity.Entity) error {
	if x.capture == nil {
		return nil
	}
	return x.capture.entityDelete(ctx, e)
}

// captureRelationDelete records a synchronous pre-delete relation version;
// same hard-error contract as captureEntityDelete.
func (x *Exec) captureRelationDelete(ctx context.Context, rel *entity.Relation) error {
	if x.capture == nil {
		return nil
	}
	return x.capture.relationDelete(ctx, rel)
}

// capturer performs the synchronous version captures, carrying the render
// projection (the projection schema_versions content-addresses — version
// rows always reference the RENDER schema, not the migration shape) and the
// attribution resolved once per file.
type capturer struct {
	w           VersionCapture
	schemaHash  string
	projection  []byte
	user, tool  string
	triggeredBy string
}

func (r *Runner) newCapturer(fileName string) *capturer {
	if r.deps.Versions == nil {
		return nil
	}
	proj := r.deps.Meta.RenderProjection()
	projJSON, err := proj.JSON()
	if err != nil {
		slog.Error("datamigration.capture_projection_marshal_failed", "error", err)
		return nil
	}
	return &capturer{
		w:           r.deps.Versions,
		schemaHash:  proj.Hash(),
		projection:  projJSON,
		user:        principal.SystemUser(),
		tool:        migrationTool,
		triggeredBy: fileName,
	}
}

func (c *capturer) entityDelete(ctx context.Context, e *entity.Entity) error {
	err := c.w.WriteVersion(ctx, store.VersionInput{
		EntityID:      e.ID,
		Op:            store.VersionOpDelete,
		Type:          e.Type,
		Content:       e.Content,
		Properties:    e.Properties,
		SchemaHash:    c.schemaHash,
		Projection:    c.projection,
		PrincipalUser: c.user,
		PrincipalTool: c.tool,
		TriggeredBy:   c.triggeredBy,
	})
	if err != nil {
		return fmt.Errorf("pre-delete version capture for %s: %w", e.ID, err)
	}
	return nil
}

func (c *capturer) relationDelete(ctx context.Context, rel *entity.Relation) error {
	err := c.w.WriteRelationVersion(ctx, store.RelationVersionInput{
		From:          rel.From,
		Type:          rel.Type,
		To:            rel.To,
		Op:            store.VersionOpDelete,
		Content:       rel.Content,
		Properties:    rel.Properties,
		SchemaHash:    c.schemaHash,
		Projection:    c.projection,
		PrincipalUser: c.user,
		PrincipalTool: c.tool,
		TriggeredBy:   c.triggeredBy,
	})
	if err != nil {
		return fmt.Errorf("pre-delete version capture for %s--%s--%s: %w", rel.From, rel.Type, rel.To, err)
	}
	return nil
}
