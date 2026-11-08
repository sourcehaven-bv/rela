package appbuild

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/rootfs"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
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
// What still touches the disk: config reads under root, and the host config
// (mail.yaml, ai.yaml, secrets) read from root/.rela unless [WithHostConfig]
// replaces it.
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
	return newInMemory(Config{
		FS:            fs,
		Paths:         paths,
		ScriptEngine:  script.NewEngine(),
		Audit:         audit.NewMemory(),
		projectConfig: rootfs.New(paths.Root),
	}, opts)
}

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
	st := memstore.New(memstore.WithObserver(idx))
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
