package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowedits"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/kvflowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// flowRunsLockFile is the lock file of the KV run store in the cache
// directory.
const flowRunsLockFile = "flow-runs.lock"

// readFlowsFile returns flows.yaml through r, or nil when it is absent.
func readFlowsFile(ctx context.Context, r configReader) ([]byte, error) {
	data, err := r.Load(ctx, flowcfg.FileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", flowcfg.FileName, err)
	}
	return data, nil
}

// withFlowRuns adds the `_flow-run` type to meta when data declares flows,
// so the ACL injects the engine role when the policy is validated next.
func withFlowRuns(data []byte, meta *metamodel.Metamodel) (*metamodel.Metamodel, error) {
	if !flowcfg.DeclaresFlows(data) {
		return meta, nil
	}
	m, err := metamodel.WithFlowRuns(meta)
	if err != nil {
		return nil, fmt.Errorf("flows: %w", err)
	}
	return m, nil
}

// compileFlows compiles flows.yaml for the base. An invalid file fails the
// boot, as realtime.yaml does. Nil when the file declares no flows.
func compileFlows(data []byte, meta *metamodel.Metamodel, policy *acl.Policy) (*flowcfg.Compiled, error) {
	if !flowcfg.DeclaresFlows(data) {
		return nil, nil //nolint:nilnil // no flows: no engine
	}
	c, err := flowcfg.Parse(data, meta, policy)
	if err != nil {
		return nil, err
	}
	for _, w := range c.Warnings() {
		slog.Warn("flows.yaml: " + w.String())
	}
	return c, nil
}

// flowRunBackend returns the run store for st: the recipe's override
// (sqlite), the store's own (postgres, see storeFlowRunsFor), else the KV
// store over the node-local cache directory.
func flowRunBackend(fsys storage.FS, paths *project.Context, st store.Store,
	override flowrun.Store) (flowrun.Store, error) {
	if override != nil {
		return override, nil
	}
	if s := storeFlowRunsFor(st); s != nil {
		return s, nil
	}
	localKV, err := buildStateKV(fsys, paths)
	if err != nil {
		return nil, err
	}
	if _, nop := localKV.(nopKV); nop || localKV == nil {
		slog.Warn("flows: no cache directory; flow runs are kept in this process only")
		return kvflowrun.New(memoryKV(), filelock.ProcessPrivate{})
	}
	var locker kvflowrun.Locker = filelock.ProcessPrivate{}
	if _, mem := fsys.(*storage.MemFS); !mem {
		if locker, err = filelock.New(filepath.Join(paths.CacheDir, flowRunsLockFile)); err != nil {
			return nil, err
		}
	}
	return kvflowrun.New(localKV, locker)
}

// buildFlowEngine builds the engine over s, or returns nil when the base
// declares no flows.
func buildFlowEngine(s *Services, runs flowrun.Store) (*flowengine.Engine, error) {
	flows := s.base.flows
	if flows == nil || runs == nil {
		return nil, nil //nolint:nilnil // no flows: no engine
	}
	d := s.aclDeclarative
	reader, _, _ := scriptReads(s.store, d, s.fieldReads, s.worlds)
	bind := identityBinderFor(d)
	as := func(ctx context.Context, user string) (context.Context, error) {
		ctx = principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolFlow})
		if d == nil {
			return ctx, nil
		}
		return bind(ctx)
	}
	var policy *acl.Policy
	if d != nil {
		policy = d.Policy()
	}
	meta := s.meta
	return flowengine.New(flowengine.Deps{
		Flows: flows, Meta: meta, Runs: lockFlowRuns(s.store, runs),
		Headers:  flowHeaders{mgr: s.entityManager, as: as},
		Outputs:  flowOutputs{mgr: entitymanager.FieldGated(s.entityManager), as: as},
		Subjects: flowSubjects{st: s.store, rows: reader, as: as},
		Compile: func(name string, src []byte, version string) (*flowcfg.Flow, error) {
			return flowcfg.ParseFlow(name, src, version, meta, policy)
		},
		Now: s.base.clock(),
	})
}

// openFlowEdits opens the run store into overrides.flowRuns and builds the
// flow edit ceiling over it, or returns nil when no flow declares
// outside_edits. The result is a nil interface, not a typed nil, so a
// caller's nil check holds.
func openFlowEdits(base *SharedBase, st store.Store, overrides *backendOverrides) (affordances.FlowEditCeiling, error) {
	if base.flows == nil {
		return nil, nil //nolint:nilnil // no flows: no ceiling, and not an error
	}
	runs, err := flowRunBackend(base.cfg.FS, base.cfg.Paths, st, overrides.flowRuns)
	if err != nil {
		return nil, fmt.Errorf("flows: %w", err)
	}
	overrides.flowRuns = runs
	c, err := flowedits.New(base.flows, base.meta, lockFlowRuns(st, runs))
	if err != nil {
		return nil, fmt.Errorf("flows: %w", err)
	}
	if c == nil {
		return nil, nil //nolint:nilnil // no flow declares outside_edits: no ceiling
	}
	return c, nil
}

// flowRunAliases moves a subject's runs to its new id when it is renamed.
// The run store keys runs by subject id. [flowRenameGuard] refuses to rename
// a subject that a run holds, inside the rename's transaction, so on
// postgres and sqlite no run is left to move. On the fs and memory tiers the
// guard and a run start are not ordered, and this move is the backstop for a
// run that started while the rename ran. Deletes are left to the runtime.
type flowRunAliases struct {
	runs flowrun.Store
}

// newFlowRunAliases returns the subscriber for runs.
//
// Nil: runs is accepted (no flows) and gives a nil subscriber, which the
// alias fanout drops.
func newFlowRunAliases(runs flowrun.Store) *flowRunAliases {
	if runs == nil {
		return nil
	}
	return &flowRunAliases{runs: runs}
}

// EntityRenamed moves the runs on oldID to newID. The rename has
// happened, so the move does not depend on the requester staying
// connected.
func (a *flowRunAliases) EntityRenamed(ctx context.Context, oldID, newID string) error {
	if _, err := a.runs.RekeySubject(context.WithoutCancel(ctx), oldID, newID); err != nil {
		return fmt.Errorf("flows: move runs from %s to %s: %w", oldID, newID, err)
	}
	return nil
}

// EntityDeleted keeps the runs; ending them is the runtime's job.
func (*flowRunAliases) EntityDeleted(context.Context, string) error { return nil }

// EntityFaceDeleted keeps the runs: they follow the subject, which still
// exists.
func (*flowRunAliases) EntityFaceDeleted(context.Context, string, entity.Face) error { return nil }

// flowRenameGuard refuses to rename a subject that an unfinished flow run
// holds (TKT-XI7IDBS). The run keeps the subject's old id, so a rename would
// separate the subject from its run and lift the run's `outside_edits:`. A
// run is deleted when it ends, so every stored run counts.
//
// The manager asks inside the rename's store transaction. On postgres and
// sqlite a run start writes under the same lock (see [txFlowRuns]), so the
// check sees every run that started before the rename.
type flowRenameGuard struct {
	runs flowrun.Store
}

// flowRenameGuardFor returns the guard over runs, which reads on the
// rename's transaction where st has one to join (see [txFlowRuns]). The
// result is a nil interface, not a typed nil, so the manager's nil check
// holds.
//
// Nil: runs is accepted (no flows) and gives no guard.
func flowRenameGuardFor(st store.Store, runs flowrun.Store) entitymanager.RenameGuard {
	if runs == nil {
		return nil
	}
	return flowRenameGuard{runs: lockFlowRuns(st, runs)}
}

// CheckRename implements [entitymanager.RenameGuard]. It fails closed: a run
// store it cannot read refuses the rename. The cause goes to the log, not
// to the caller, because it can carry database detail.
func (g flowRenameGuard) CheckRename(ctx context.Context, id string) error {
	held, err := g.runs.List(ctx, flowrun.Query{SubjectID: id, Limit: 1})
	if err != nil {
		slog.ErrorContext(ctx, "flows: rename refused: the flow runs could not be read", "id", id, "err", err)
		return fmt.Errorf("%w: the flow runs on %s could not be read", entitymanager.ErrRenameBlocked, id)
	}
	if len(held) > 0 {
		return fmt.Errorf("%w: an unfinished flow run holds %s; finish or cancel the run first",
			entitymanager.ErrRenameBlocked, id)
	}
	return nil
}

// FlowEdits returns the flow edit ceiling of s, for the data-entry
// resolver (see dataentry.ResolverFromServices). It is a package function
// rather than a method because [Services] is at its plimsoll cap.
//
// Nil: returned when no flow declares outside_edits.
func FlowEdits(s *Services) affordances.FlowEditCeiling { return s.flowEdits }

// FlowEngine returns the flow engine of s. It is a package function rather
// than a method because [Services] is at its plimsoll cap.
//
// Nil: returned when flows.yaml declares no flows.
func FlowEngine(s *Services) *flowengine.Engine { return s.flowEngine }

// asIdentity stamps ctx with the principal user, tool flow, bound to the
// policy.
type asIdentity func(ctx context.Context, user string) (context.Context, error)

// flowHeaders writes run headers as system:flow through the entity manager,
// so the managed-type guard, the audit log and the ACL's engine role apply.
type flowHeaders struct {
	mgr *entitymanager.Manager
	as  asIdentity
}

func (h flowHeaders) engine(ctx context.Context) (context.Context, error) {
	return h.as(ctx, principal.UserFlow)
}

// CreateHeader creates the header and its `_carries` edge in one store
// transaction. Inside it, before anything is written, it checks that the
// subject still exists, so a start that races a delete leaves no header on
// a store that rolls back (RR-DTSDBII).
func (h flowHeaders) CreateHeader(ctx context.Context, subject entity.Ref,
	props map[string]any) (flow.RunID, error) {
	ctx, err := h.engine(ctx)
	if err != nil {
		return "", err
	}
	res, err := entitymanager.CreateLinked(ctx, h.mgr, entitymanager.LinkedCreate{
		Entity:     &entity.Entity{Type: entity.FlowRunType, Properties: props},
		RelateType: entity.CarriesRelation, RelateTo: subject.String(),
		BeforeWrite: func(ctx context.Context, view store.Store) error {
			if _, gerr := view.GetEntity(ctx, subject); gerr != nil {
				if errors.Is(gerr, store.ErrNotFound) {
					return flowengine.ErrSubjectNotFound
				}
				return gerr
			}
			return nil
		},
	})
	if err != nil {
		var kept *entitymanager.KeptEntityError
		if errors.As(err, &kept) {
			h.deleteQuietly(ctx, kept.Entity.ID)
		}
		return "", err
	}
	if res.FollowUpErr != nil {
		slog.WarnContext(ctx, "flows: run header follow-up failed", "run", res.Create.Entity.ID,
			"error", res.FollowUpErr)
	}
	return flow.RunID(res.Create.Entity.ID), nil
}

func (h flowHeaders) deleteQuietly(ctx context.Context, id string) {
	if _, err := h.mgr.DeleteEntity(ctx, id, true); err != nil {
		slog.WarnContext(ctx, "flows: a run header without a run was left behind", "run", id, "error", err)
	}
}

func (h flowHeaders) PatchHeader(ctx context.Context, id flow.RunID, props map[string]any) error {
	ctx, err := h.engine(ctx)
	if err != nil {
		return err
	}
	_, err = h.mgr.PatchEntity(ctx, string(id), entity.Patch{Properties: props})
	return err
}

func (h flowHeaders) DeleteHeader(ctx context.Context, id flow.RunID) error {
	ctx, err := h.engine(ctx)
	if err != nil {
		return err
	}
	_, err = h.mgr.DeleteEntity(ctx, string(id), true)
	return err
}

func (h flowHeaders) LinkProduced(ctx context.Context, id flow.RunID, created entity.Ref) error {
	ctx, err := h.engine(ctx)
	if err != nil {
		return err
	}
	_, err = h.mgr.CreateRelation(ctx, entity.RelationKey{
		From: string(id), FromFace: entity.ImplicitFace, Type: entity.ProducedRelation, To: created.ID,
	}, entity.RelationOptions{})
	return err
}

// flowOutputs writes a state's outputs as the flow's run_as identity
// through the field-gated manager: the row ACL, the field write gate,
// validation, automations and the audit apply as to any write.
type flowOutputs struct {
	mgr *entitymanager.Manager
	as  asIdentity
}

// writer refuses rela's own identities as a run_as, so an output never
// writes as the process. flowcfg refuses them at load too.
func (o flowOutputs) writer(ctx context.Context, runAs string) (context.Context, error) {
	if runAs == "" || principal.IsRelaIdentity(runAs) {
		return ctx, fmt.Errorf("flows: run_as %q may not write outputs", runAs)
	}
	return o.as(ctx, runAs)
}

// Set writes the output of state as run's own write: the outside_edits of
// that state do not narrow it, every other run's and state's still do
// (TKT-XI7IDBS).
func (o flowOutputs) Set(
	ctx context.Context, runAs string, run flow.RunID, state string, subject entity.Ref, props map[string]any,
) error {
	ctx, err := o.writer(ctx, runAs)
	if err != nil {
		return err
	}
	if len(props) == 0 {
		return nil
	}
	_, err = o.mgr.PatchEntity(flowedits.WithinRun(ctx, run, state), subject.String(), entity.Patch{Properties: props})
	return err
}

func (o flowOutputs) Create(ctx context.Context, runAs, typ string, props map[string]any, relate string,
	subject entity.Ref) (entity.Ref, error) {
	ctx, err := o.writer(ctx, runAs)
	if err != nil {
		return entity.Ref{}, err
	}
	lc := entitymanager.LinkedCreate{Entity: &entity.Entity{Type: typ, Properties: props}}
	if relate != "" {
		lc.RelateType, lc.RelateTo = relate, subject.String()
	}
	res, err := entitymanager.CreateLinked(ctx, o.mgr, lc)
	if err != nil {
		var kept *entitymanager.KeptEntityError
		if errors.As(err, &kept) {
			// The entity is written; only its relation or audit failed.
			// Counting the output as done keeps a retry from a duplicate.
			slog.WarnContext(ctx, "flows: output created, but a follow-up write failed",
				"entity", kept.Entity.ID, "error", kept.Err)
			return kept.Entity.Ref(), nil
		}
		return entity.Ref{}, err
	}
	if res.FollowUpErr != nil {
		slog.WarnContext(ctx, "flows: output follow-up failed", "entity", res.Create.Entity.ID,
			"error", res.FollowUpErr)
	}
	return res.Create.Entity.Ref(), nil
}

// flowSubjects reads subjects: raw for the engine's guards, through the
// gated reader as run_as for templates.
type flowSubjects struct {
	st   store.Store
	rows interface {
		GetAddress(ctx context.Context, addr string) (*entity.Entity, error)
	}
	as asIdentity
}

func (s flowSubjects) Subject(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	e, err := s.st.GetEntity(ctx, ref)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", flowengine.ErrSubjectNotFound, ref.String())
	}
	return e, err
}

func (s flowSubjects) SubjectAs(ctx context.Context, runAs string, ref entity.Ref) (map[string]any, error) {
	ctx, err := s.as(ctx, runAs)
	if err != nil {
		return nil, err
	}
	e, err := s.rows.GetAddress(ctx, ref.String())
	switch {
	case err == nil:
		return e.Properties, nil
	case errors.Is(err, store.ErrNotFound), errors.Is(err, entitymanager.ErrEntityNotFound):
		// A row run_as cannot read reads as missing.
		return map[string]any{}, nil
	default:
		return nil, err
	}
}
