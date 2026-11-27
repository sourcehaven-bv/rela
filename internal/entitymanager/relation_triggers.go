package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrTriggerDepthExceeded refuses a relation write that would raise a
// relation event after its chain of triggers has used [relationEventBudget]
// dispatches. A trigger that keeps creating the relation it watches stops
// here instead of looping.
var ErrTriggerDepthExceeded = errors.New("entitymanager: relation trigger chain exceeded its limit")

// relationEventBudget is how many relation-event dispatches one write and
// every write its triggers cause may make together. It counts dispatches,
// not nesting, so it also bounds a trigger that fans out. It equals the
// cascade's own bound, so a chain stops after the same number of steps.
const relationEventBudget = autocascade.MaxDepth

type relationBudgetKey struct{}

// spendRelationBudget takes one dispatch from the budget on ctx when a
// trigger watches ev for one of relTypes, and returns ctx carrying the
// budget. The first spend in a chain puts a fresh budget on ctx. A write
// that raises no watched event spends nothing.
func spendRelationBudget(
	ctx context.Context, deps Deps, ev triggers.EventType, relTypes ...string,
) (context.Context, error) {
	watched := false
	for _, t := range relTypes {
		if watchesRelation(deps, ev, t) {
			watched = true
			break
		}
	}
	if !watched {
		return ctx, nil
	}
	b, _ := ctx.Value(relationBudgetKey{}).(*atomic.Int32)
	if b == nil {
		b = new(atomic.Int32)
		b.Store(relationEventBudget)
		ctx = context.WithValue(ctx, relationBudgetKey{}, b)
	}
	if b.Add(-1) < 0 {
		return ctx, ErrTriggerDepthExceeded
	}
	return ctx, nil
}

// withRelationBudget puts the budget of from on ctx.
func withRelationBudget(ctx, from context.Context) context.Context {
	if b, ok := from.Value(relationBudgetKey{}).(*atomic.Int32); ok {
		return context.WithValue(ctx, relationBudgetKey{}, b)
	}
	return ctx
}

// watchesRelation reports whether a trigger in deps watches ev for relType.
func watchesRelation(deps Deps, ev triggers.EventType, relType string) bool {
	return deps.Automations != nil && deps.Automations.WatchesRelation(ev, relType)
}

// ErrRelationTriggerCheck refuses a relation write whose trigger raised an
// error: a `check` of severity error that the source row fails, or an
// output that cannot run as written.
var ErrRelationTriggerCheck = errors.New("entitymanager: relation trigger refused the write")

// fireRelationEvent raises ev for rel inside the write's transaction, on
// view. The subject is the relation's source: the face row of a tailed
// edge, or every row of the source family the caller may read for an
// identity edge. A trigger error refuses the write. Each row's `set` and
// `unset` outputs are written to that row on view, after the caller's own
// update authorization for the row, so they commit or roll back with the
// relation. The rest of each row's outputs run as a cascade after the commit.
func fireRelationEvent(
	ctx context.Context, m *Manager, view store.Store, ev triggers.EventType, rel *entity.Relation,
) error {
	if !watchesRelation(m.deps, ev, rel.Type) {
		return nil
	}
	rows, err := relationSourceRows(ctx, m, view, rel)
	if err != nil {
		return err
	}
	for _, row := range rows {
		res := m.deps.Automations.Process(ctx, triggers.Event{Type: ev, Entity: row, OldEntity: row, Relation: rel})
		where := entity.FormatStateRef(row.ID, row.Face)
		if res.HasErrors() {
			return fmt.Errorf("%w: %s trigger on %s: %s",
				ErrRelationTriggerCheck, rel.Type, where, strings.Join(res.Errors, "; "))
		}
		subject := row
		if res.HasWrites() {
			if aErr := authorizeTriggerWrite(ctx, m, row); aErr != nil {
				return aErr
			}
			if subject, err = writeRelationTriggerSet(ctx, m.deps, row, res); err != nil {
				return fmt.Errorf("%s trigger on %s: %w", rel.Type, where, err)
			}
		}
		if hasCascadeWork(res) && !onCommit(ctx, relationCascade(ctx, m, subject, res, rel)) {
			return errors.New("entitymanager: relation trigger outside a manager transaction")
		}
	}
	return nil
}

// authorizeTriggerWrite asks whether the caller of m may update row, as
// [Manager.AuthorizeUpdate] does. A trigger writes the source row on the
// caller's behalf, and a relation grant does not cover that update. For a
// cascade manager only the managed-type rule applies, as for every cascade
// write.
func authorizeTriggerWrite(ctx context.Context, m *Manager, row *entity.Entity) error {
	if err := requireReadableRow(ctx, faceGateOf(m), row.ID, []*entity.Entity{row}); err != nil {
		return err
	}
	return m.authorizeAndAudit(ctx, entityUpdateRequest(row.Type, row.Ref()))
}

// relationSourceRows reads the rows a relation event on rel fires for, on
// view. Rows the caller of m cannot read are left out, so a trigger on a
// caller's own relation write never writes a face the caller could not see.
// A cascade manager (cascadeWrite) has no read gate and gets every row: a
// relation a cascade creates fires its triggers as a system action.
func relationSourceRows(
	ctx context.Context, m *Manager, view store.Store, rel *entity.Relation,
) ([]*entity.Entity, error) {
	if rel.FromFace != "" {
		row, err := view.GetEntity(ctx, entity.Ref{ID: rel.From, Face: rel.FromFace})
		if err != nil {
			return nil, fmt.Errorf("read source of %s: %w", rel.Type, err)
		}
		return readableRows(ctx, faceGateOf(m), []*entity.Entity{row})
	}
	fam, err := lookupFamily(ctx, view, rel.From)
	if err != nil {
		return nil, fmt.Errorf("read source of %s: %w", rel.Type, err)
	}
	rows := make([]*entity.Entity, 0, len(fam.faces))
	for _, face := range fam.faces {
		row, gErr := view.GetEntity(ctx, entity.Ref{ID: fam.id, Face: face})
		if gErr != nil {
			return nil, fmt.Errorf("read source of %s: %w", rel.Type, gErr)
		}
		rows = append(rows, row)
	}
	return readableRows(ctx, faceGateOf(m), rows)
}

// writeRelationTriggerSet applies the property writes of res to stored, a
// row read on the transaction view ctx carries, and returns the row as
// written. It runs the checks [Manager.PatchEntity] runs on an
// automation's values: computed, external-ref and file properties are
// refused, decimals are stored as text, validation errors refuse the write
// as the deployment's [ValidationMode] says, and the state machine judges
// the move from stored. Unique, write-enforced and relation rules run
// through [writeChecked], which joins view, so a relation rule judges the
// written row against the edges the relation write leaves. The write is
// pinned to the version read, which nothing else can change inside the
// transaction. Its audit record names the rule that wrote.
func writeRelationTriggerSet(
	ctx context.Context, deps Deps, stored *entity.Entity, res *triggers.Result,
) (*entity.Entity, error) {
	if stored.IsLocked() {
		return nil, fmt.Errorf("entity %s has inaccessible fields", stored.ID)
	}
	writes := res.Writes()
	if err := rejectComputedPresent(deps, stored.Type, writeNames(writes)); err != nil {
		return nil, err
	}
	if err := rejectExternalRefPresent(deps.Meta, stored.Type, writeNames(writes)); err != nil {
		return nil, err
	}
	next := stored.Clone()
	writes.Apply(next)
	storeDecimalsAsText(deps.Meta, next.Type, next.Properties)
	if err := deps.Computed.Evaluate(ctx, next); err != nil {
		return nil, err
	}
	v := metamodel.NewValidator(deps.Meta)
	hard, _ := partitionValidationErrors(deps.ValidationMode, v.ValidateEntity(next.ID, next.Type, next.Properties))
	if len(hard) > 0 {
		return nil, newValidationError(hard)
	}
	err := deps.Transitions.EnforceUpdate(ctx, stored, next, deps.TransitionGuard, deps.TransitionGraph)
	if err != nil {
		return nil, err
	}
	ctx = setByCtx(ctx, res.SetBy)
	m := &Manager{deps: deps, cascadeWrite: true}
	pinned := store.UpdateCondition{ExpectedVersion: store.VersionOf(stored)}
	summary := updateEntitySummary(stored, next)
	txCheck := relationRuleCheck(deps, stored, next)
	err = writeChecked(ctx, deps, next, next.ID, txCheck, func(ctx context.Context, st store.Store) error {
		if _, uErr := st.UpdateEntityIf(ctx, next, pinned); uErr != nil {
			return uErr
		}
		if aErr := m.recordEntityAudit(ctx, st, audit.OpUpdateEntity, next, summary); aErr != nil {
			return aErr
		}
		return runUpdateHook(ctx, deps, st, stored, next)
	})
	if err != nil {
		if ok, mapped := mapUniquePropertyConflict(err); ok {
			return nil, mapped
		}
		return nil, err
	}
	return next, nil
}

// setByCtx labels ctx with the rule that wrote, as the cascade runner
// labels its writes: `automation:<name>` for one rule, `automation` when
// several wrote. A label already on ctx wins.
func setByCtx(ctx context.Context, setBy []string) context.Context {
	if audit.TriggeredByFrom(ctx) != "" {
		return ctx
	}
	if len(setBy) == 1 && setBy[0] != "" {
		return audit.WithTriggeredBy(ctx, "automation:"+setBy[0])
	}
	return audit.WithTriggeredBy(ctx, "automation")
}

// hasCascadeWork reports whether res holds an output the cascade runs.
func hasCascadeWork(res *triggers.Result) bool {
	return len(res.RelationsToCreate) > 0 || len(res.EntitiesToCreate) > 0 || len(res.LuaToExecute) > 0 ||
		len(res.PilesToPush) > 0 || len(res.Notifications) > 0 || len(res.Warnings) > 0
}

// relationCascade returns the cascade of one relation event, to run after
// the commit. It runs on the caller's ctx with the chain's budget, so a
// relation the cascade writes spends from the same budget. The relation
// write returns only an error, so the cascade's errors and warnings are
// logged, not returned.
func relationCascade(
	fired context.Context, m *Manager, subject *entity.Entity, res *triggers.Result, rel *entity.Relation,
) func(context.Context) {
	return func(ctx context.Context) {
		ctx = withRelationBudget(ctx, fired)
		outcome, err := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
			Trigger:    subject,
			OldTrigger: subject,
			Result:     res,
			Scripts:    m.deps.ScriptRunner,
			Mutator:    m.gated(),
		})
		if err != nil {
			outcome.Errors = append(outcome.Errors, err.Error())
		}
		for _, msg := range outcome.Errors {
			slog.WarnContext(ctx, "entitymanager: relation trigger error",
				"relation", relationLabel(rel), "error", msg)
		}
		for _, msg := range outcome.Warnings {
			slog.WarnContext(ctx, "entitymanager: relation trigger warning",
				"relation", relationLabel(rel), "warning", msg)
		}
	}
}

// relationLabel formats rel for a log line.
func relationLabel(rel *entity.Relation) string {
	return fmt.Sprintf("%s --%s--> %s", entity.FormatStateRef(rel.From, rel.FromFace), rel.Type, rel.To)
}
