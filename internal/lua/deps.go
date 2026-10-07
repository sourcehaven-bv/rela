package lua

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rootfs"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// EntityReader is the read-OUT surface every graph-reading Lua binding
// goes through. Defined here at the consumer per CLAUDE.md "interfaces at
// the call site" (same rationale as [Mutator] and cacheStore) — three
// methods, which is exactly what the bindings call.
//
// The wiring site decides what backs it, and that decision IS the read-ACL
// (DEC-O59WM4): a visibility-backed adapter binds reads to the acting
// identity, while the operator-trust-boundary paths (CLI, docs runtime)
// take the ungated visibility reader. Bindings therefore contain no ACL
// logic at all — they cannot forget to gate, because they have nothing else
// to read through.
type EntityReader interface {
	// GetAddress reads an entity ADDRESS (`ID` or `ID@face`), not an id: a
	// bare id resolves in the reader's world. A store does not satisfy this
	// method, because its GetEntity takes an entity.Ref.
	GetAddress(ctx context.Context, addr string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// ReadDeps is the capability bundle required to run a read-only Lua runtime.
// A runtime built from ReadDeps (see NewReader) exposes only query, trace,
// search, schema introspection, and output-to-stdout bindings. It cannot
// mutate the graph and cannot write files — both rela.create_entity et al.
// and rela.write_file are absent from the rela.* table on a reader.
//
// ProjectRoot is the absolute project path; used by writer runtimes to
// resolve the output directory for rela.write_file.
type ReadDeps struct {
	// VisibleReader serves every READ-OUT: rela.get_entity,
	// rela.list_entities, rela.get_relations, rela.search hydration, and
	// markdown entity-refs. Whatever the wiring site puts here defines what
	// scripts can see.
	//
	// A nil VisibleReader DENIES (the binding raises); it never falls back
	// to a raw handle. A forgotten wiring must not become an ACL bypass
	// (RR-X9NVHI).
	//
	// There is deliberately NO second, raw store field beside this one.
	// rela.update_entity used to need one for its read-before-write — and
	// that field was a standing hazard, since pointing it at VisibleReader
	// by mistake made the save erase whatever the caller could not see.
	// The binding now builds an [entity.Patch] and lets the manager merge
	// against the raw stored entity internally (TKT-80EWGM), so no Lua
	// binding holds an ungated read path and the mistake is unavailable.
	// Elevated reads are the sole exception and stay explicitly opt-in via
	// [WriteDeps.ElevatedReader].
	VisibleReader EntityReader

	// Tracer is the graph-traversal surface. Wiring injects either the
	// plain tracer or a visibility decorator over it; the trace bindings
	// are identical either way (they carry no properties, so gating is
	// purely node/title pruning inside the decorator).
	Tracer tracer.Tracer

	// Searcher produces search hits. rela.search hydrates each hit through
	// VisibleReader and drops the ones it cannot read, so no hidden entity
	// or property reaches the script. Whether the hit list itself is gated
	// depends on the wiring: appbuild's GatedReads (remote MCP) supplies a
	// visibility.Searcher, which also drops matches on hidden properties;
	// the other wirings supply the raw searcher, where a timing/count
	// residual remains (the TKT-GGQ0JT class of oracle).
	Searcher search.Searcher

	Meta        *metamodel.Metamodel
	ProjectRoot string

	// Files serves the project's operator-authored script files — scripts/,
	// actions/, validations/ — by slash path relative to the project root.
	// Read them through [ReadDeps.ReadProjectFile], never from disk directly:
	// on the sqlite build they may live in the project's database
	// (FEAT-UP14BT), layered behind the files on disk.
	//
	// Nil: accepted — files are read from ProjectRoot on disk with os.Root
	// containment, which is what every build did before the database could
	// carry them.
	Files ProjectFiles

	// Host supplies secrets and the AI and mail settings to writer runtimes
	// (see [LoadContextOptions]).
	//
	// Nil: accepted — they are read from ProjectRoot/.rela, which is what
	// every build did before the desktop kept them elsewhere.
	Host HostConfig

	// Capabilities declares the ambient, non-graph capabilities a runtime
	// built from these deps may reach — outbound HTTP, the AI provider, named
	// secrets, and rela.write_file (TKT-YH52OM).
	//
	// The zero value grants NOTHING, which is the point: before this existed
	// every runtime on every surface held http, ai and the entire contents of
	// .rela/secrets.yaml, so any script could read a secret and POST it out in
	// two calls. A forgotten wiring must deny, exactly as a nil VisibleReader
	// denies rather than falling back to a raw handle (RR-X9NVHI).
	//
	// It lives on ReadDeps rather than WriteDeps because READ-only surfaces
	// (validation rules, document renders) had the same exposure — they cannot
	// mutate the graph, but they could still exfiltrate.
	//
	// Precedence: a NON-EMPTY [WithCapabilities] overrides this; an empty one
	// leaves it alone. Engine.execute passes that option unconditionally, so
	// treating an empty grant as a revocation silently erased this field for
	// every plain ExecuteCode/ExecuteFile caller — which is how the scheduler
	// runs. See the WithCapabilities godoc.
	Capabilities Capabilities

	// World is the world script list reads resolve in: rela.list_entities,
	// admin.list_entities and rela.md.entity_refs list each entity at the
	// face this world serves. Wiring passes worlds.Compiled.DefaultWorld. The
	// zero value is unset, and a list read in it fails with
	// store.ErrInvalidQuery rather than reading the trivial world.
	World store.WorldScope
}

// ProjectFiles reads operator-authored project files by slash path relative
// to the project root. config.Loader and rootfs.Dir satisfy it. An absent file
// is reported as fs.ErrNotExist.
type ProjectFiles interface {
	Load(ctx context.Context, name string) ([]byte, error)
}

// ReadProjectFile returns the project file at name through [ReadDeps.Files],
// or from ProjectRoot with os.Root containment when Files is nil.
func (d ReadDeps) ReadProjectFile(ctx context.Context, name string) ([]byte, error) {
	return d.projectFiles().Load(ctx, name)
}

func (d ReadDeps) projectFiles() ProjectFiles {
	if d.Files == nil {
		return rootfs.New(d.ProjectRoot)
	}
	return d.Files
}

// ReadScript returns the source of the Lua script at name under the project
// directory dir (scripts, actions or validations).
//
// name must be a local path ending in .lua. A backslash separates path
// elements on every platform, so a name reads the same file on Windows as on
// the Unix server or in the database. Errors name the script but never a
// filesystem path, so they are safe to return to an HTTP or MCP caller.
func (d ReadDeps) ReadScript(ctx context.Context, dir, name string) (string, error) {
	return ReadScript(ctx, d.projectFiles(), dir, name)
}

// ReadScript is [ReadDeps.ReadScript] over files, for callers that read a
// script without running it and so hold no read bundle.
//
// Nil: files is rejected.
func ReadScript(ctx context.Context, files ProjectFiles, dir, name string) (string, error) {
	if files == nil {
		return "", errors.New("no project files to read the script from")
	}
	if name == "" {
		return "", errors.New("script path is empty")
	}
	slashed := strings.ReplaceAll(name, `\`, "/")
	if !fs.ValidPath(slashed) || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("script path must be a local path (no '..' or absolute paths): %s", name)
	}
	if !strings.HasSuffix(name, ".lua") {
		return "", fmt.Errorf("script must have .lua extension: %s", name)
	}
	data, err := files.Load(ctx, dir+"/"+slashed)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("script not found: %s (must be in %s/ directory)", name, dir)
	}
	if err != nil {
		return "", fmt.Errorf("cannot read script: %s", name)
	}
	return string(data), nil
}

// SourceFS returns the project files as an fs.FS, for reading the source
// excerpt of a script error ([BuildInput.SourceFS]).
func (d ReadDeps) SourceFS(ctx context.Context) fs.FS {
	return projectFilesFS{ctx: ctx, files: d.projectFiles()}
}

// projectFilesFS adapts ProjectFiles to fs.FS. Only files can be opened.
type projectFilesFS struct {
	ctx   context.Context //nolint:containedctx // fs.FS's Open takes no ctx.
	files ProjectFiles
}

func (f projectFilesFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	data, err := f.files.Load(f.ctx, name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &projectFile{name: path.Base(name), Reader: bytes.NewReader(data), size: int64(len(data))}, nil
}

// projectFile is an open, in-memory [projectFilesFS] file.
type projectFile struct {
	*bytes.Reader
	name string
	size int64
}

func (f *projectFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *projectFile) Close() error               { return nil }
func (f *projectFile) Name() string               { return f.name }
func (f *projectFile) Size() int64                { return f.size }
func (f *projectFile) Mode() fs.FileMode          { return projectFileMode }
func (f *projectFile) ModTime() time.Time         { return time.Time{} }
func (f *projectFile) IsDir() bool                { return false }
func (f *projectFile) Sys() any                   { return nil }

// projectFileMode is the mode a [projectFile] reports: readable, never
// writable through this view.
const projectFileMode fs.FileMode = 0o444

// Mutator is the consumer-side write surface Lua bindings call into
// from rela.create_entity / rela.update_entity / rela.delete_entity /
// rela.create_relation / rela.delete_relation. Defined here at the
// consumer per CLAUDE.md "interfaces at the call site"; the wiring
// site supplies an implementation (the production one being the
// project's EntityManager).
//
// Six methods — RenameEntity and UpdateRelation are intentionally
// absent because no Lua binding invokes them. Narrowed from the
// wider EntityManager interface in TKT-IF37 to drop lua's transitive
// dependency on internal/entitymanager.
//
// PatchEntity is what rela.update_entity is built on: the binding names
// the properties the script touched and nothing else, so it needs no raw
// store handle of its own to read-modify-write through (TKT-80EWGM).
type Mutator interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	DeleteEntityFace(ctx context.Context, id string, face entity.Face, cascade bool) (*entity.DeleteResult, error)
	CreateRelation(ctx context.Context, key entity.RelationKey, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, key entity.RelationKey) error
}

// NotFoundError is an OPTIONAL capability a [Mutator]'s returned error may
// implement to say "the target entity does not exist". Declared here at the
// consumer so lua needs no dependency on internal/entitymanager (same
// rationale as [Mutator] itself); the production error type satisfies it.
//
// Why this exists rather than a string match: several hard errors embed
// caller-supplied values. An illegal state-machine transition formats the
// attempted value with %q, so `strings.Contains(err.Error(), "entity not
// found")` misreports a *rejected transition* as a *missing entity* when a
// script sets a property to that literal text — and a script branching on
// that message could try to recreate a row that still exists. Structural
// beats textual.
type NotFoundError interface {
	error
	// EntityNotFound reports that the write failed because the target
	// entity does not exist, as opposed to any other hard error.
	EntityNotFound() bool
}

// isEntityNotFound reports whether err (or anything it wraps) declares
// itself an entity-not-found condition.
func isEntityNotFound(err error) bool {
	var nfe NotFoundError
	return errors.As(err, &nfe) && nfe.EntityNotFound()
}

// WriteDeps is the capability bundle required to run a read-write Lua runtime.
// A runtime built from WriteDeps (see NewWriter) additionally exposes
// create/update/delete bindings for entities and relations.
type WriteDeps struct {
	ReadDeps
	EntityManager Mutator

	// ElevatedManager, when non-nil, is a write handle whose mutations skip
	// the ACL deny (TKT-D8T148). It is set ONLY for an allow_acl_bypass
	// automation action.
	//
	// Since TKT-Y3JVFK it is no longer the sole key to rela.bypass_acl:
	// EITHER this or ElevatedReader registers the binding. What each handle
	// controls is which METHODS the `admin` table carries — with this nil the
	// write methods are absent entirely, so a reader-only elevation cannot
	// mutate. Both nil ⇒ no binding, and a script cannot elevate at all.
	ElevatedManager Mutator

	// ElevatedReader, when non-nil, is the RAW read handle backing the
	// admin.get_entity / list_entities / get_relations methods of the
	// bypass_acl closure (TKT-ACSBSA). Reads through it are unredacted and
	// ungated — the closure is the boundary, so a half-elevated read would
	// be a confusing contract.
	//
	// This is now the ONLY raw read handle a writer runtime can carry, and
	// it is opt-in. Previously a second raw field (WritePrepStore) was
	// present on EVERY writer runtime for update_entity's read-before-write;
	// removing it (TKT-80EWGM) means an ungated read path now exists only
	// where elevation was explicitly requested.
	//
	// It may be set WITHOUT ElevatedManager (TKT-Y3JVFK) — a READ-ONLY
	// elevation, which is how a document render aggregates over rows its
	// caller cannot see while remaining structurally unable to mutate (the
	// `admin` table simply has no write methods). On the cascade path both
	// handles are still set together, at the same site under the same two
	// conditions.
	//
	// Nil is a DENY, not a fallback: admin.get_entity raises rather than
	// silently reading through the gated VisibleReader. Elevation that
	// quietly degrades to the caller's view is worse than a loud failure —
	// the script would see a partial graph and treat it as complete.
	ElevatedReader EntityReader

	// ElevationRecorder, when non-nil, is notified once per bypass_acl
	// closure that performed at least one elevated READ (TKT-ACSBSA).
	//
	// Elevated WRITES are already audited inside entitymanager
	// (audit.OpACLBypass). Reads never reach entitymanager — they go
	// straight to the store — so without this an operator querying the
	// audit log for elevation sees every elevated write and is silently
	// blind to every elevated read. That asymmetry is the gap this closes.
	//
	// ONCE PER CLOSURE, not per read: a single admin.list_entities can
	// traverse the whole graph, and a per-row audit record would put an
	// unbounded synchronous write on a read path. The forensic question an
	// operator actually asks is "which elevated scopes read raw data", and
	// the closure is the unit of elevation.
	//
	// Nil disables recording. Unlike ElevatedReader, nil here is NOT a deny:
	// a wiring site may legitimately have no audit sink (CLI, tests), and
	// refusing to run automations because of that would be a worse failure
	// than an unrecorded read.
	ElevationRecorder ElevationRecorder

	// VersionTags backs rela.tag_version and rela.untag_version
	// (TKT-VO6VG9). It is the gated write path: it authorizes the tag
	// for the ctx principal and audits it.
	//
	// Nil: accepted, and the two bindings are then absent. Set it only where
	// a principal deliberately runs a script, such as a data-entry action or
	// a scheduled script. The synchronous automation cascade, document
	// renders, export_render and the data-entry webhook leave it nil.
	VersionTags VersionTagWriter
}

// ElevationRecorder receives a notification when a rela.bypass_acl closure
// used its elevated READ capability. Defined here at the consumer per
// CLAUDE.md "interfaces at the call site" — one method, which is all the
// binding needs; the wiring site adapts it onto the audit sink.
type ElevationRecorder interface {
	// RecordElevatedRead is called at most once per closure, after it
	// returns, when at least one elevated read occurred. bindings names the
	// distinct admin read methods used (e.g. "get_entity,list_entities") so
	// the record says what kind of access happened without recording the
	// data itself.
	//
	// Implementations must not block: this runs on the script's goroutine
	// inside the cascade. Errors are the implementation's to handle — the
	// binding has no way to surface them and a failed audit write must not
	// fail the automation.
	RecordElevatedRead(ctx context.Context, bindings []string)
}
