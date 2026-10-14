// Package ingest runs an [ingestspec.Mapping] against one inbound record:
// find the entity it concerns, create it when missing, then apply the steps.
// The declarative webhook endpoint is one caller; it keeps the HTTP concerns
// (body cap, header allowlist, load shedding, principal, status codes) and
// hands each delivery here.
//
// # Concurrency
//
// Records run concurrently, in one process and across processes. Both halves
// of the pipeline detect a racing writer rather than relying on a lock:
//
//   - CREATE loses on the `unique:` constraint, which the manager checks and
//     writes inside one store.Tx (and pgstore also backs with a derived
//     index), and the retry loop re-finds the winner.
//   - APPEND is a compare-and-swap. Patch.Content is an ABSOLUTE replacement
//     computed from a base read moments earlier, so the patch is pinned to that
//     base's version; a racing writer turns the write into a
//     *store.VersionConflictError and the retry loop re-runs the attempt.
//
// Re-running the WHOLE attempt (re-find, re-derive, re-apply) is what makes the
// retry correct: it re-reads fresh state rather than replaying a decision made
// against stale state. This is why steps must stay pure.
//
// # Attribution
//
// Run authorizes and audits as the principal on ctx; it sets none itself. A
// caller attaches its own (principal.With) before calling, so the audit log
// names the endpoint or subscription that produced the write.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Actions a [Result] reports.
const (
	ActionCreated = "created"
	ActionUpdated = "updated"
	ActionNoMatch = "no_match"
)

// ErrConflictExhausted reports that the retry budget was spent without a
// clean write. Distinct from a generic failure so a caller can tell the
// producer to retry rather than report a fault.
var ErrConflictExhausted = errors.New("ingest: conflict retries exhausted")

// maxPauseShift caps the retry pause window at 1ms<<5 = 32ms.
const maxPauseShift = 5

// Source supplies the values a mapping's templates reference.
type Source interface {
	// Resolve returns the RAW value of one {{ref}}, or "" when absent.
	// [Interpolate] flattens it; a Source must not be asked to.
	Resolve(ref string) string
	// DefaultMatchRef returns the REFERENCE (not a template) a find.match
	// property with no find.values entry reads, such as "body.<prop>" for a
	// webhook. A reference cannot carry record data into template position:
	// its value goes through Resolve and is flattened like any other.
	DefaultMatchRef(prop string) string
}

// Reader is the ACL-gated read the find step uses.
//
// GetAddress is not called. It is here because a store does not have it
// (store.Store.GetEntity takes an entity.Ref), so the raw store cannot be
// passed as a Reader by mistake; the visibility wrappers satisfy it.
type Reader interface {
	GetAddress(ctx context.Context, addr string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// Writer is the write path every mutation goes through.
type Writer interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	PatchEntity(ctx context.Context, id string, patch entity.Patch) (*entity.UpdateResult, error)
}

// WritePrepReader reads the raw stored row an append is spliced onto.
type WritePrepReader interface {
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
}

// Deps are the collaborators one [Run] uses. A value, passed per call,
// because a caller's reader and writer may be swapped on a config reload and
// must not be captured across records.
type Deps struct {
	// Reader serves the find. It must be the caller's ACL-gated reader: a
	// denied entity then reads as a miss, so the mapping is not an existence
	// oracle, and a principal that cannot see the match creates a new entity
	// rather than silently updating an invisible one.
	Reader Reader
	// World is the world a faceless find type is read in.
	World store.WorldScope
	// Writer performs the create and the patch.
	Writer Writer
	// WritePrep is the RAW store, used only for the append base re-read. A
	// redacted base would be written back over the stored body.
	WritePrep WritePrepReader
	// MaxAttempts bounds the conflict-retry loop. Every round has a winner, so
	// N concurrent records to one entity need at most N rounds; a caller sets
	// it to the concurrency it admits.
	MaxAttempts int
	// Label names the mapping in log lines.
	Label string
}

func (d Deps) validate() error {
	switch {
	case d.Reader == nil:
		return errors.New("ingest: no visible reader configured")
	case d.Writer == nil:
		return errors.New("ingest: no writer configured")
	case d.WritePrep == nil:
		return errors.New("ingest: no write-prep reader configured")
	case d.MaxAttempts < 1:
		return fmt.Errorf("ingest: MaxAttempts %d must be at least 1", d.MaxAttempts)
	}
	return nil
}

// Result is the outcome of one record.
type Result struct {
	Action   string // ActionCreated | ActionUpdated | ActionNoMatch
	EntityID string // empty for ActionNoMatch
}

// Run executes find → create-if-missing → then-steps for one record under
// the conflict-retry budget.
func Run(ctx context.Context, d Deps, m ingestspec.Mapping, src Source) (Result, error) {
	if err := d.validate(); err != nil {
		return Result{}, err
	}
	var lastErr error
	for attempt := range d.MaxAttempts {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		result, err := runAttempt(ctx, d, m, src)
		if err == nil {
			return result, nil
		}
		if !IsConflict(err) {
			return Result{}, err
		}
		lastErr = err
		slog.Debug("ingest write conflict, retrying",
			"mapping", d.Label, "attempt", attempt+1, "error", err)
		if attempt+1 < d.MaxAttempts {
			if err := retryPause(ctx, attempt+1); err != nil {
				return Result{}, err
			}
		}
	}
	return Result{}, fmt.Errorf("%w: %w", ErrConflictExhausted, lastErr)
}

// retryPause waits a short random time before the next attempt, so losers do
// not collide again in lockstep; the window doubles per attempt up to 32ms.
// It returns ctx's error if ctx ends first.
func retryPause(ctx context.Context, attempt int) error {
	window := time.Millisecond << min(attempt, maxPauseShift)
	//nolint:gosec // G404: jitter, not a secret
	t := time.NewTimer(rand.N(window))
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runAttempt is one pass of the pipeline.
func runAttempt(ctx context.Context, d Deps, m ingestspec.Mapping, src Source) (Result, error) {
	target, err := findTarget(ctx, d, m, src)
	if err != nil {
		return Result{}, err
	}

	action := ActionUpdated
	if target == nil {
		if m.CreateIfMissing == nil {
			// Find-and-update-only with no match: a legitimate no-op, not an
			// error. Reported so the producer can tell it apart from a write.
			return Result{Action: ActionNoMatch}, nil
		}
		created, createErr := createEntity(ctx, d, m, src)
		if createErr != nil {
			return Result{}, createErr
		}
		target = created
		action = ActionCreated
	}

	if len(m.Then) > 0 {
		if err := applySteps(ctx, d, m, src, target); err != nil {
			return Result{}, err
		}
	}
	return Result{Action: action, EntityID: target.ID}, nil
}

// interpolate expands tmpl against src. Every template goes through here so
// that every value is flattened.
func interpolate(src Source, tmpl string) string {
	return Interpolate(tmpl, src.Resolve)
}

// matchExpr is the template for one find.match property.
func matchExpr(m ingestspec.Mapping, src Source, prop string) string {
	if expr, ok := m.Find.Values[prop]; ok {
		return expr
	}
	return "{{" + src.DefaultMatchRef(prop) + "}}"
}

// findTarget resolves the entity a record concerns, or nil when there is no
// match (or no find: block at all).
//
// Nil: returned, with a nil error, as the NO-MATCH result. The caller branches
// to create on it; a sentinel error would have to be unwrapped at the one call
// site to mean the same thing.
//
//nolint:nilnil // (nil, nil) IS the no-match result; see above.
func findTarget(
	ctx context.Context, d Deps, m ingestspec.Mapping, src Source,
) (match *entity.Entity, err error) {
	if m.Find == nil || len(m.Find.Match) == 0 {
		return nil, nil
	}

	want := make(map[string]string, len(m.Find.Match))
	for _, prop := range m.Find.Match {
		value := interpolate(src, matchExpr(m, src, prop))
		if value == "" {
			// An empty match value would match every entity missing that
			// property. Refusing is the safe direction: it degrades to
			// "create a new one", never "update an arbitrary one".
			return nil, nil
		}
		want[prop] = value
	}

	for e, listErr := range d.Reader.ListEntities(ctx, store.EntityQuery{
		Type: m.Find.Type, Faces: findFaces(m.Find, d.World),
	}) {
		if listErr != nil {
			return nil, fmt.Errorf("ingest find: %w", listErr)
		}
		if e == nil || !matchesAll(e, want) {
			continue
		}
		// Deterministic winner when the operator's schema does not enforce
		// uniqueness: lowest ID. Without this, which duplicate a record
		// appends to would vary per record and the history would interleave.
		if match == nil || e.ID < match.ID {
			match = e
		}
	}
	return match, nil
}

// findFaces is the face selection a find reads. A faced type is read at
// find.face, the face the mapping writes, which config load requires; a
// faceless type in the caller's world.
func findFaces(find *ingestspec.Find, world store.WorldScope) store.FaceSelection {
	if find.Face != "" {
		return store.AtFaces(entity.Face(find.Face))
	}
	return store.InWorld(world)
}

// matchesAll reports whether e has every wanted property value.
func matchesAll(e *entity.Entity, want map[string]string) bool {
	for prop, value := range want {
		if propString(e, prop) != value {
			return false
		}
	}
	return true
}

// propString renders a property as the string a match compares against.
func propString(e *entity.Entity, prop string) string {
	v, ok := e.Properties[prop]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// createEntity creates the entity for a find-miss.
//
// No GetEntity pre-check and no uniqueness scan of our own: that would be a
// TOCTOU duplicate of the store's atomic guarantee, which is exactly the
// reasoning Manager.CreateEntity already records. A racing duplicate surfaces
// as a unique violation and is retried by [Run], which re-finds and then takes
// the update path.
func createEntity(ctx context.Context, d Deps, m ingestspec.Mapping, src Source) (*entity.Entity, error) {
	spec := m.CreateIfMissing
	createType := m.CreateType()
	if createType == "" {
		return nil, errors.New("ingest: create_if_missing has no resolvable entity type")
	}

	e := &entity.Entity{
		Type:       createType,
		Properties: make(map[string]any, len(spec.Properties)),
	}
	for prop, expr := range spec.Properties {
		e.Properties[prop] = interpolate(src, expr)
	}
	// Seed the match values too, so an entity created by a find-or-create
	// mapping is findable by the SAME key on the next record even when the
	// operator listed the property only under find.match. Explicit properties
	// win.
	if m.Find != nil {
		for _, prop := range m.Find.Match {
			if _, already := e.Properties[prop]; already {
				continue
			}
			if v := interpolate(src, matchExpr(m, src, prop)); v != "" {
				e.Properties[prop] = v
			}
		}
	}
	if spec.Content != "" {
		e.Content = interpolate(src, spec.Content)
	}

	opts := entity.CreateOptions{Variant: spec.Template}
	if m.Find != nil && e.Type == m.Find.Type {
		opts.Face = entity.Face(m.Find.Face)
	}
	res, err := d.Writer.CreateEntity(ctx, e, opts)
	if err != nil {
		return nil, fmt.Errorf("ingest create: %w", err)
	}
	if res == nil || res.Entity == nil {
		return nil, errors.New("ingest: create returned no entity")
	}
	return res.Entity, nil
}

// applySteps runs the then: steps against target.
//
// Every mutation goes through PatchEntity naming ONLY what it changes. That is
// the CLAUDE.md rule and it matters especially here: the find read is
// ACL-gated and may be redacted, so a read-modify-write through UpdateEntity
// would carry a redacted view back to the store and erase every property the
// mapping's principal could not see.
//
// # Why the body is re-read here
//
// A `set` step names properties, so PatchEntity's own write-prep read is the
// merge base and nothing can be lost. An `append_section` step cannot work that
// way: Patch.Content is an ABSOLUTE replacement, so the appended body has to be
// computed from some base, which makes it a read-modify-write and therefore the
// one step that CAN lose a concurrent write.
// TestWebhookConflict_BlindUpdateLosesAppends (internal/dataentry)
// demonstrates that loss against postgres, to document what is being avoided.
//
// The re-read is the base the append is computed from, and its version is the
// patch's ExpectedVersion: the compare-and-swap the package doc describes.
//
// On the CREATE path the re-read matters for a second reason: `target` there
// is the entity as constructed, whose Content does not reflect what templates
// and on-create automations actually persisted. Splicing onto the stored body
// is what keeps a template-provided section from being dropped.
func applySteps(
	ctx context.Context, d Deps, m ingestspec.Mapping, src Source, target *entity.Entity,
) error {
	// Accumulate into one patch so a multi-step mapping performs a single write.
	patch := entity.Patch{}
	content := target.Content
	contentChanged := false

	if m.NeedsBody() {
		fresh, err := d.WritePrep.GetEntity(ctx, target.Ref())
		if err != nil {
			return fmt.Errorf("ingest re-read body: %w", err)
		}
		content = fresh.Content
		patch.ExpectedVersion = string(store.VersionOf(fresh))
	}

	for i, step := range m.Then {
		switch {
		case step.AppendSection != nil:
			// Interpolate flattens each substituted value, so the only
			// newlines reaching here are ones the OPERATOR wrote in the
			// template: those are theirs to make, and to get wrong.
			block := interpolate(src, step.AppendSection.Content)
			content = markdown.AppendToSection(content, step.AppendSection.Section, block)
			contentChanged = true
		case len(step.Set) > 0:
			if patch.Properties == nil {
				patch.Properties = make(map[string]any, len(step.Set))
			}
			for prop, expr := range step.Set {
				patch.Properties[prop] = interpolate(src, expr)
			}
		default:
			return fmt.Errorf("ingest: then[%d] declares no action", i)
		}
	}

	if contentChanged {
		patch.Content = &content
	}
	if patch.IsEmpty() {
		return nil
	}

	if _, err := d.Writer.PatchEntity(ctx, target.Ref().String(), patch); err != nil {
		return fmt.Errorf("ingest apply: %w", err)
	}
	return nil
}

// IsConflict reports whether err is a write conflict worth retrying: a
// uniqueness collision on create, or a store conflict (the shape a lost
// append compare-and-swap arrives as).
func IsConflict(err error) bool {
	// Matching on the unique validation error, not only the store error, is
	// what makes the retry loop reachable at all: the manager's pre-write scan
	// raises it with no store error to wrap.
	//
	// Deliberately narrow: ONLY a unique collision counts. Any other
	// validation failure ("status must be one of...") is a genuine rejection
	// that re-running would reproduce forever, so it must NOT be retried.
	if entitymanager.IsUniqueViolation(err) {
		return true
	}
	// ErrConflict also matches *store.VersionConflictError, a lost append.
	return errors.Is(err, store.ErrConflict)
}
