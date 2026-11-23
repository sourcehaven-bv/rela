package appbuild

import (
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/idproles"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// SchedulerWorkspace is the capability bundle the scheduler runs against: it
// satisfies scheduler.WorkspaceProvider and every optional capability the
// scheduler type-asserts (job queue, metamodel, audit, raw store, for_each
// fan-out, mail templates). Build it with [Services.Scheduler].
//
// It holds copies of the collaborators it needs rather than a *Services, so
// the scheduler cannot reach the rest of the wiring facade (TKT-N0IKN9).
type SchedulerWorkspace struct {
	paths          *project.Context
	cfgLoader      config.Loader
	stateKV        state.KV
	schedulerState schedulerstate.Store
	jobQueue       jobs.Client
	meta           *metamodel.Metamodel
	audit          audit.Audit
	store          store.Store
	worlds         worlds.Compiled
	aclPolicy      *acl.Policy
	idpRoles       *idproles.Store
	mail           *mailRuntime
	// clock is the assembly's clock ([WithClock]), or the wall clock.
	clock func() time.Time
	// writeDeps builds the per-task Lua bundle. A closure because the bundle
	// is assembled from most of Services; see [SchedulerWorkspace.ScheduledLuaWriteDeps].
	writeDeps func() lua.WriteDeps
}

// The scheduler type-asserts every capability past WorkspaceProvider, so a
// missing method would silently disable a feature rather than fail to
// compile. These pin each one; the anonymous interface mirrors the
// scheduler's unexported jobQueueProvider, metaProvider, auditProvider and
// rawStoreProvider.
var (
	_ scheduler.WorkspaceProvider = (*SchedulerWorkspace)(nil)
	_ scheduler.ForEachProvider   = (*SchedulerWorkspace)(nil)
	_ scheduler.TemplateRunner    = (*SchedulerWorkspace)(nil)
	_ interface {
		Jobs() jobs.Client
		Meta() *metamodel.Metamodel
		Audit() audit.Audit
		Store() store.Store
	} = (*SchedulerWorkspace)(nil)
)

// Scheduler returns the scheduler's view of s. It is built on each call and
// copies s's collaborators at that moment, so call it after assembly.
//
// Nil: never returned.
func (s *Services) Scheduler() *SchedulerWorkspace {
	return &SchedulerWorkspace{
		paths:          s.paths,
		cfgLoader:      s.cfgLoader,
		stateKV:        s.stateKV,
		schedulerState: s.schedulerState,
		jobQueue:       s.jobQueue,
		meta:           s.meta,
		audit:          s.audit,
		store:          s.store,
		worlds:         s.worlds,
		aclPolicy:      s.aclPolicy,
		idpRoles:       s.idpRoles,
		mail:           s.mail,
		clock:          servicesClock(s),
		writeDeps: func() lua.WriteDeps {
			deps := s.luaWriteDepsFor(s.fieldReads)
			deps.EntityManager = FieldGatedEntityManager(s)
			return deps
		},
	}
}

// Paths returns the project paths.
func (w *SchedulerWorkspace) Paths() *project.Context { return w.paths }

// Config returns the project's config loader.
func (w *SchedulerWorkspace) Config() config.Loader { return w.cfgLoader }

// State returns the .rela cache-directory KV, which holds the legacy
// scheduler-state.json.
func (w *SchedulerWorkspace) State() state.KV { return w.stateKV }

// Meta returns the loaded metamodel.
func (w *SchedulerWorkspace) Meta() *metamodel.Metamodel { return w.meta }

// Audit returns the audit sink.
func (w *SchedulerWorkspace) Audit() audit.Audit { return w.audit }

// Store returns the raw store, for the scheduler's write-prep reads.
func (w *SchedulerWorkspace) Store() store.Store { return w.store }

// Jobs returns the background-job queue, already started.
//
// The return type deliberately omits [jobs.Lifecycle]: starting and stopping
// the queue is the composition root's concern, and a consumer that could Close
// it would be shutting background work down for every other subsystem. Services
// keeps the lifecycle handle privately and tears it down in Close.
//
// Register handlers at wiring time; the dispatcher resolves a handler per job,
// so registration after start is fine. Durability depends on the build —
// ephemeral on fs/desktop, durable on postgres.
//
// Nil: never — assemble fails rather than returning a Services with no
// queue, since a nil queue would turn every Enqueue into a panic at the
// call site rather than a wiring error here.
func (w *SchedulerWorkspace) Jobs() jobs.Client { return w.jobQueue }

// SchedulerState returns the scheduler's task and run records. The backend
// matches the job queue's reach: postgres when the queue is durable, kvstate
// over [Services.State] otherwise.
func (w *SchedulerWorkspace) SchedulerState() schedulerstate.Store { return w.schedulerState }

// ScheduledLuaWriteDeps returns the per-task Lua bundle. Reads are ACL-bound
// to the task's principal (stamped on the per-task ctx), with privileges
// coming from acl.yaml — a job sees what its identity may see, nothing more
// (DEC-O59WM4).
//
// Field-level `visible:` redaction APPLIES here (TKT-0XL8MF): a job whose
// identity may read `person` receives that entity with the same properties
// redacted as a human with the same role sees in the UI. This closed
// RR-7408F5, which documented the earlier row-gating-only behavior.
//
// Field write grants apply too: writes go through the [entitymanager.FieldGated]
// handle, so a job cannot set a property its identity may not.
func (w *SchedulerWorkspace) ScheduledLuaWriteDeps() lua.WriteDeps { return w.writeDeps() }
