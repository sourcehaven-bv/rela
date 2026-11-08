package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/rootfs"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// NewInMemory builds a services bundle for the project at root that never
// writes to disk. It reads the project's config from disk and keeps every
// write in memory.
//
// The world is built as follows:
//   - The store is a memstore searched by [search.LinearSearch]. It starts
//     empty: the entities and relations on disk are not loaded.
//   - Config.FS is a fresh [storage.MemFS]. Everything appbuild writes through
//     it lands in memory: the state KV, the migration record, comments,
//     piles, leases, channels and the IdP role document. Lock files for an
//     in-memory FS are process-private, so none is created on disk.
//   - Operator-authored config (schema.yaml, acl.yaml, scripts, templates,
//     data-entry.yaml) is read from root through [rootfs], the seam the
//     sqlite recipe uses for config carried in its database.
//   - Audit records go to an [audit.Memory].
//
// The index must not be bleve: this file is untagged, so the postgres build
// links it, and that build must link no bleve (see .claude/rules/storage.md).
// Search ranking is therefore LinearSearch's, which differs from bleve and
// the database backends.
//
// [Services.FS] returns the MemFS, not the project directory. Code that needs
// the operator files reads them through the config loader instead.
//
// Every call runs a full [prepare] and [assemble], so each world gets its own
// options and shares nothing mutable with another. The caller must call
// [Services.Close], which stops the background workers the world started.
//
// The world is safe by default. Without options it:
//   - sends mail to a [mail.MemorySender] that the caller cannot read, so
//     mail is discarded, never sent over the network. mail.yaml is still
//     read, so its recipients policy applies. Pass [WithMailTransport] to
//     inspect what was sent;
//   - refuses outbound HTTP and AI calls from Lua; the error names
//     [ErrRealNetwork];
//   - reads no secrets and no ai.yaml: the host config serves mail.yaml only;
//   - points rela.write_file at a new temporary directory outside root, which
//     [Services.Close] removes.
//
// Each default gives way to the matching option: [WithMailTransport],
// [WithHTTPClient], [WithAIProvider], [WithHostConfig], [WithOutputDir].
// A caller that passes one takes responsibility for what it injects; an
// output directory it names is not removed on Close. A nil or empty option
// is ignored, so it keeps the default.
//
// The defaults reach every Lua runtime and evaluator these services build:
// automations, scheduled tasks, scripts, actions run through
// [Services.LuaReadDeps], validation and write rules. A host that builds its
// own runtimes from its own lua.ReadDeps does not get them; the data-entry
// app's action and validation runtimes are such a host. Mounting that app on
// a world needs its runtimes wired from the world's deps first.
//
// It fails when the temporary directory would be inside root, as when
// TMPDIR points into the project.
//
// What still touches the disk: config reads under root, mail.yaml read from
// root/.rela, and the temporary output directory.
func NewInMemory(root string, opts ...Option) (*Services, error) {
	if root == "" {
		return nil, errors.New("appbuild.NewInMemory: project root is required")
	}
	fs := storage.NewMemFS()
	// Resolve the paths against the disk, where the schema is; the MemFS
	// holds nothing yet.
	paths, err := project.At(root, storage.NewOsFS())
	if err != nil {
		return nil, fmt.Errorf("open project at %s: %w", root, err)
	}
	defaults, cleanup, err := inMemoryDefaults(paths, opts)
	if err != nil {
		return nil, err
	}
	svc, err := newInMemory(Config{
		FS:            fs,
		Paths:         paths,
		ScriptEngine:  script.NewEngine(),
		Audit:         audit.NewMemory(),
		projectConfig: rootfs.New(paths.Root),
	}, append(defaults, opts...))
	if err != nil {
		cleanup()
		return nil, err
	}
	svc.onClose = cleanup
	return svc, nil
}

// ErrRealNetwork is the error an in-memory world's default HTTP client
// returns; its default AI provider returns an [ai.Error] with the same
// message. [NewInMemory] refuses real calls unless the caller injects a
// client or provider.
var ErrRealNetwork = errors.New("real HTTP and AI calls are not allowed in an in-memory world")

// inMemoryDefaults returns the safe-by-default options of [NewInMemory]
// (see its godoc), placed before the caller's so the caller's win, and a
// cleanup that removes what they created. A default the caller replaces is
// not created at all.
func inMemoryDefaults(paths *project.Context, opts []Option) ([]Option, func(), error) {
	var set options
	for _, opt := range opts {
		opt(&set)
	}
	defaults := []Option{
		WithMailTransport(mail.NewMemorySender(0)),
		WithHTTPClient(&http.Client{Transport: refusingTransport{}}),
		WithAIProvider(refusingAIProvider{}),
		WithHostConfig(func(state.KV) (HostConfig, error) {
			return mailOnlyHost{dir: hostconfig.Dir(paths.CacheDir)}, nil
		}),
	}
	cleanup := func() {}
	if set.seams.outputDir == "" {
		dir, err := os.MkdirTemp("", "rela-world-output-")
		if err != nil {
			return nil, nil, fmt.Errorf("appbuild.NewInMemory: output directory: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(dir) }
		if inside(paths.Root, dir) {
			cleanup()
			return nil, nil, fmt.Errorf("appbuild.NewInMemory: the temporary directory %s is inside the project", dir)
		}
		defaults = append(defaults, WithOutputDir(dir))
	}
	return defaults, cleanup, nil
}

// inside reports whether path is root or below it, after resolving symlinks
// in both, so /var and /private/var on macOS compare equal.
func inside(root, path string) bool {
	rel, err := filepath.Rel(resolved(root), resolved(path))
	return err == nil && filepath.IsLocal(rel)
}

// resolved is path with its symlinks resolved, or path itself when it does
// not exist.
func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return path
}

// refusingTransport is the HTTP transport of an in-memory world: every
// request fails with [ErrRealNetwork] before any connection is made.
type refusingTransport struct{}

// RoundTrip refuses the request.
func (refusingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, ErrRealNetwork
}

// refusingAIProvider is the AI provider of an in-memory world: every call
// fails with a network-kind [ai.Error] carrying [ErrRealNetwork]'s message.
// The Lua binding only reports a typed [ai.Error] to the script.
type refusingAIProvider struct{}

// Chat refuses the call.
func (refusingAIProvider) Chat(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	return nil, refusedAI()
}

// Embed refuses the call.
func (refusingAIProvider) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return nil, refusedAI()
}

// refusedAI is the error every refusingAIProvider call returns.
func refusedAI() *ai.Error {
	return &ai.Error{Kind: ai.ErrNetwork, Message: ErrRealNetwork.Error()}
}

// mailOnlyHost is the host config of an in-memory world. It serves mail.yaml
// from the project's .rela directory, so the recipients policy and base URL
// apply, and nothing else: ai.yaml reads as absent and there are no secrets.
// A test world must not reach the credentials of the machine it runs on.
type mailOnlyHost struct{ dir hostconfig.Dir }

// File serves mail.yaml and reports every other file as absent.
func (h mailOnlyHost) File(name string) ([]byte, error) {
	if name != hostconfig.MailFile {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	return h.dir.File(name)
}

// Secrets reports that there are none.
func (mailOnlyHost) Secrets(string) (map[string]string, error) { return nil, secrets.ErrNotFound }

// Path is the .rela directory, which mail.yaml paths resolve against.
func (h mailOnlyHost) Path() string { return h.dir.Path() }

// newInMemory is the in-memory recipe over a caller-built Config: a memstore
// with a LinearSearch index wired as a write observer. LinearSearch fills
// itself through the observer hook, and the store starts empty, so there is
// nothing to backfill.
//
// [NewInMemory] calls it with a hermetic Config. The memorybackend build's
// New calls it with the Config [Discover] built, so that build keeps writing
// its runtime state under .rela/ as before.
func newInMemory(cfg Config, opts []Option) (*Services, error) {
	base, err := prepare(cfg, opts)
	if err != nil {
		return nil, err
	}
	idx := search.NewLinearSearch()
	st := memstore.New(memstore.WithObserver(idx), memstore.WithClock(base.clock()))
	// nil VisibleSearcher: assemble derives the generic search.NewVisible
	// wrapper (TKT-BA8BSX). Only the postgres recipe has a native one.
	return assemble(base, st, search.New(st, idx), nil, idx, backendOverrides{})
}

// inMemoryFS reports whether fs is an in-memory filesystem, which no other
// process can open. Every site that would otherwise take a lock file under
// the cache directory asks this, and takes a process-private lock instead
// when it is true: lock files are written with os calls, not through fs, so
// they would land on disk even when fs is in memory.
func inMemoryFS(fs storage.FS) bool {
	_, mem := fs.(*storage.MemFS)
	return mem
}
