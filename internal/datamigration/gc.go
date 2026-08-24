package datamigration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// gcTool is the attribution/audit identity of GC deletions.
const gcTool = "gc-sweep"

// DefaultGrace is how long orphaned data survives after the schema deletion
// was first observed before a GC pass may delete it. The window is the
// safety net for the rename-ambiguity hole: a rename mistaken for a
// delete+add costs eventual, recoverable loss — never immediate loss.
const DefaultGrace = 30 * 24 * time.Hour

// VerdictSource yields the latest gate verdict — consumer-side slice of
// *Gate. The GC engine reads it EVERY tick (the gate re-evaluates on
// metamodel hot-reload, so a cached verdict could be stale).
type VerdictSource interface {
	Verdict() *Verdict
}

// GCDeps are the GC engine's collaborators. All but Versions are required —
// notably Audit (amendment A8): an engine that deletes data without an audit
// trail must be unconstructable, not misconfigurable.
type GCDeps struct {
	Store    store.Store
	Meta     func() *metamodel.Metamodel // latest metamodel (hot-reload aware)
	State    state.KV
	Audit    audit.Audit
	Verdicts VerdictSource
	Versions VersionCapture // optional (pg only)
	Grace    time.Duration  // 0 = DefaultGrace
	// Lock serializes GC applies against migration runs and other GC
	// writers (TKT-CPCBR7); build it with [LockFor]. Required — the one
	// unattended data-deleting path must not lose its serialization.
	Lock MigrationLock
}

// GC deletes schema-orphaned data recorded in the drift ledger once the
// grace period has passed. It is invoked three ways, all through the same
// engine: the server's periodic sweep goroutine, `rela migrate gc` (manual),
// and a scheduler task on headless projects.
type GC struct {
	deps GCDeps
	now  func() time.Time
}

// NewGC validates required collaborators up front.
func NewGC(deps GCDeps) (*GC, error) {
	switch {
	case deps.Store == nil:
		return nil, errors.New("datamigration: NewGC: Store is required")
	case deps.Meta == nil:
		return nil, errors.New("datamigration: NewGC: Meta is required")
	case deps.State == nil:
		return nil, errors.New("datamigration: NewGC: State is required")
	case deps.Audit == nil:
		return nil, errors.New("datamigration: NewGC: Audit is required")
	case deps.Verdicts == nil:
		return nil, errors.New("datamigration: NewGC: Verdicts is required")
	case deps.Lock == nil:
		return nil, errors.New("datamigration: NewGC: Lock is required (use LockFor)")
	}
	if deps.Grace <= 0 {
		deps.Grace = DefaultGrace
	}
	return &GC{deps: deps, now: time.Now}, nil
}

// GCResult reports one tick.
type GCResult struct {
	// Skipped is non-empty when the tick did nothing, with the reason
	// ("gate not evaluated yet", "needs-migration pending").
	Skipped string
	// Deleted lists the ledger entries whose data was removed this tick
	// (or would be, in dry-run).
	Deleted []GCEntry
	// Pending lists entries still inside the grace period.
	Pending []GCEntry
	Applied bool
}

// GCEntry is one ledger entry in a tick report.
type GCEntry struct {
	Key       string
	Kind      string
	Affected  int
	FirstSeen time.Time
	Deadline  time.Time
}

// Tick runs one GC pass. It NEVER touches data while the gate reports
// needs-migration deltas — a pending migration may be about to transform
// exactly the data an expired ledger entry names. Ledger entries whose
// subject the live schema declares again are dropped without touching data.
func (g *GC) Tick(ctx context.Context, apply bool) (*GCResult, error) {
	res := &GCResult{Applied: apply}
	v := g.deps.Verdicts.Verdict()
	if v == nil {
		res.Skipped = "gate has not evaluated yet"
		return res, nil
	}
	if v.Status == StatusNeedsMigration {
		res.Skipped = "schema needs migration — GC will not touch data a pending migration may transform"
		return res, nil
	}
	if apply {
		// An apply tick competes with migration runs and other GC writers;
		// contention is a skip, not an error — for the sweep the other
		// holder is doing equivalent work, and for the CLI the message names
		// the situation honestly.
		release, lockErr := g.deps.Lock.TryAcquire(ctx)
		if errors.Is(lockErr, ErrLockHeld) {
			res.Skipped = "another migration or GC run holds the migration lock"
			return res, nil
		}
		if lockErr != nil {
			return nil, lockErr
		}
		defer release()
	}

	live := g.deps.Meta().ShapeProjection()
	ledger, err := LoadLedger(ctx, g.deps.State)
	if err != nil {
		return nil, err
	}
	dirty := len(ledger.PruneAgainst(live)) > 0

	now := g.now().UTC()
	expired := map[string]bool{}
	for _, key := range ledger.Expired(now, g.deps.Grace) {
		expired[key] = true
	}

	if apply {
		p := principal.From(ctx)
		user := p.User
		if user == "" {
			user = "system:" + gcTool
		}
		ctx = store.WithAttribution(ctx, store.Attribution{User: user, Tool: gcTool})
	}

	for key, e := range ledger.Entries {
		entry := GCEntry{Key: key, Kind: e.Kind, FirstSeen: e.FirstSeen, Deadline: e.FirstSeen.Add(g.deps.Grace)}
		if !expired[key] {
			res.Pending = append(res.Pending, entry)
			continue
		}
		affected, err := g.collect(ctx, key, e.Kind, apply)
		if err != nil {
			return res, fmt.Errorf("datamigration: gc %s: %w", key, err)
		}
		entry.Affected = affected
		res.Deleted = append(res.Deleted, entry)
		if apply {
			delete(ledger.Entries, key)
			dirty = true
		}
	}

	if apply && dirty {
		if err := SaveLedger(ctx, g.deps.State, ledger); err != nil {
			return res, err
		}
	}
	if apply && len(res.Deleted) > 0 {
		g.auditTick(ctx, res)
	}
	return res, nil
}

// collect deletes (or counts, in dry-run) the data one expired ledger entry
// names, reusing the migration step executors — same capture, same batching.
func (g *GC) collect(ctx context.Context, key, kind string, apply bool) (int, error) {
	x := &Exec{Store: g.deps.Store, Apply: apply, capture: g.newCapturer()}
	var (
		sr  StepResult
		err error
	)
	switch kind {
	case "property":
		owner, prop, ok := splitPropertyKey(key)
		if !ok {
			return 0, errors.New("malformed ledger key")
		}
		step := &dropPropertyStep{Entity: owner, Property: prop}
		sr, err = step.Run(ctx, x)
	case "entity_type":
		step := &dropEntitiesStep{Type: key}
		sr, err = step.Run(ctx, x)
	case "relation_type":
		step := &dropRelationsStep{Type: trimRelPrefix(key)}
		sr, err = step.Run(ctx, x)
	case "relation_property":
		owner, prop, ok := splitPropertyKey(trimRelPrefix(key))
		if !ok {
			return 0, errors.New("malformed ledger key")
		}
		sr, err = dropRelationProperty(ctx, x, owner, prop)
	default:
		// A kind this build cannot process (a legacy or future entry) must
		// not wedge the sweep forever: one poisoned entry would abort every
		// tick before any legitimate GC runs. Drop it — the ledger is
		// rebuildable bookkeeping (`rela migrate gc --scan`), never source
		// data.
		slog.Warn("datamigration.gc_unknown_ledger_kind_dropped", "key", key, "kind", kind)
		return 0, nil
	}
	return sr.Affected, err
}

// dropRelationProperty removes one orphaned property from every relation of
// a type (there is no declarative step for this; relations of a still-known
// type with a dropped property are GC-only territory).
func dropRelationProperty(ctx context.Context, x *Exec, relType, prop string) (StepResult, error) {
	res := StepResult{Kind: "drop_relation_property", Target: "rel:" + relType + "." + prop}
	rels, err := collectRelations(ctx, x.Store, relType)
	if err != nil {
		return res, err
	}
	for _, r := range rels {
		if _, has := r.Properties[prop]; !has {
			continue
		}
		res.Affected++
		if !x.Apply {
			continue
		}
		delete(r.Properties, prop)
		data := store.RelationData{Properties: r.Properties, Content: r.Content}
		if _, err := x.Store.UpdateRelation(ctx, r.From, r.Type, r.To, data); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (g *GC) newCapturer() *capturer {
	if g.deps.Versions == nil {
		return nil
	}
	meta := g.deps.Meta()
	proj := meta.RenderProjection()
	projJSON, err := proj.JSON()
	if err != nil {
		return nil
	}
	return &capturer{
		w:           g.deps.Versions,
		schemaHash:  proj.Hash(),
		projection:  projJSON,
		user:        "system:" + gcTool,
		tool:        gcTool,
		triggeredBy: gcTool,
	}
}

func (g *GC) auditTick(ctx context.Context, res *GCResult) {
	parts := make([]string, 0, len(res.Deleted))
	total := 0
	for _, d := range res.Deleted {
		parts = append(parts, fmt.Sprintf("%s (%d)", d.Key, d.Affected))
		total += d.Affected
	}
	g.deps.Audit.Record(audit.Record{
		Time:      g.now().UTC(),
		Op:        audit.OpDataGC,
		Principal: principal.From(ctx),
		Summary:   fmt.Sprintf("gc removed %d records for expired drift: %s", total, strings.Join(parts, ", ")),
	})
}

// Scan reconciles the ledger against the actual data: orphans that never
// went through a gate transition (pre-feature legacy keys, hand-edited
// files) are added with first-seen = now. It is CLI-only (`rela migrate gc
// --scan`) — a full content read is fine for an operator command, and
// deliberately NOT done by the periodic tick (amendment A6).
func (g *GC) Scan(ctx context.Context) (added []string, err error) {
	// Scan writes the ledger, so it takes the same lock as the applies.
	release, err := g.deps.Lock.TryAcquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	live := g.deps.Meta().ShapeProjection()
	ledger, err := LoadLedger(ctx, g.deps.State)
	if err != nil {
		return nil, err
	}
	now := g.now().UTC()

	record := func(key, kind string) {
		if _, exists := ledger.Entries[key]; exists {
			return
		}
		ledger.Entries[key] = LedgerEntry{Kind: kind, FirstSeen: now}
		added = append(added, key)
	}
	if err := scanEntities(ctx, g.deps.Store, live, record); err != nil {
		return nil, err
	}
	if err := scanRelations(ctx, g.deps.Store, live, record); err != nil {
		return nil, err
	}

	if len(added) > 0 {
		if err := SaveLedger(ctx, g.deps.State, ledger); err != nil {
			return nil, err
		}
	}
	return added, nil
}

func scanEntities(
	ctx context.Context, st store.Store, live metamodel.ShapeProjection, record func(key, kind string),
) error {
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			return err
		}
		es, known := live.Entities[e.Type]
		if !known {
			record(e.Type, "entity_type")
			continue
		}
		for prop := range e.Properties {
			if strings.HasPrefix(prop, "_") {
				continue // rela-managed namespace, never schema-declared
			}
			if _, ok := es.Properties[prop]; !ok {
				record(e.Type+"."+prop, "property")
			}
		}
	}
	return nil
}

func scanRelations(
	ctx context.Context, st store.Store, live metamodel.ShapeProjection, record func(key, kind string),
) error {
	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			return err
		}
		rs, known := live.Relations[r.Type]
		if !known {
			record("rel:"+r.Type, "relation_type")
			continue
		}
		for prop := range r.Properties {
			// The underscore namespace is rela-managed (e.g. the relation
			// order properties) and never schema-declared — not an orphan.
			if strings.HasPrefix(prop, "_") {
				continue
			}
			if _, ok := rs.Properties[prop]; !ok {
				record("rel:"+r.Type+"."+prop, "relation_property")
			}
		}
	}
	return nil
}
