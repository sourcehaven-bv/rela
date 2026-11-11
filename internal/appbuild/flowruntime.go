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
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
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
		Flows: flows, Meta: meta, Runs: runs,
		Headers:  flowHeaders{mgr: s.entityManager, as: as},
		Outputs:  flowOutputs{mgr: entitymanager.FieldGated(s.entityManager), as: as},
		Subjects: flowSubjects{st: s.store, rows: reader, as: as},
		Compile: func(name string, src []byte, version string) (*flowcfg.Flow, error) {
			return flowcfg.ParseFlow(name, src, version, meta, policy)
		},
	})
}

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

func (o flowOutputs) Set(ctx context.Context, runAs string, subject entity.Ref, props map[string]any) error {
	ctx, err := o.writer(ctx, runAs)
	if err != nil {
		return err
	}
	if len(props) == 0 {
		return nil
	}
	_, err = o.mgr.PatchEntity(ctx, subject.String(), entity.Patch{Properties: props})
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
