//go:build postgres

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/comments/pgcomments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/pgmigstate"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/pgschedstate"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// New builds the services bundle for the `postgres` build: a pgstore and its
// in-database search backend, both sharing one pgx pool built from the resolved
// DSN. This recipe owns only the backend choice; [prepare] and [assemble] do
// the build-agnostic work shared with the FS/memory builds.
//
// The metamodel and templates still come from the filesystem (see
// Config.Paths) — PostgreSQL backs entities/relations/attachments/search only.
// A DSN is required: Config.DatabaseURL, which Discover populates from the
// RELA_DATABASE_URL environment variable (env-only, never a flag).
func New(cfg Config, opts ...Option) (*Services, error) {
	base, err := prepare(cfg, opts)
	if err != nil {
		return nil, err
	}
	st, searcher, overrides, closer, err := openBackend(context.Background(), base)
	if err != nil {
		return nil, err
	}
	// The postgres store implements search.VisibleSearcher natively
	// (visibility composed into the search SQL — pgstore.SearchVisible,
	// TKT-BA8BSX). Assert loudly rather than silently falling back to
	// the generic wrapper: a fallback would hide a wiring regression.
	visible, ok := st.(search.VisibleSearcher)
	if !ok {
		return nil, errors.New("appbuild: postgres store does not implement search.VisibleSearcher")
	}
	return assemble(base, st, searcher, visible, closer, overrides)
}

// openBackend builds the pool this build's services share, then wires each
// consumer over it.
//
// The POOL is owned here, not by pgstore (TKT-OGTVJW). Three things read this
// database — the store, its in-database search backend, and the comment store —
// and only the composition root knows when the last of them is done, which is
// why the returned closer closes the pool. pgstore.Open used to build the pool
// itself and hand back only the store and searcher, which made it a composition
// root inside the store package and left nothing for a third consumer to share.
//
// Migration runs here for the same reason: it is a property of the database
// this process is about to use, not of any one consumer of it.
//
// The migration RECORD is a fourth consumer (TKT-XCJ0Y2): it lives in the
// tenant's schema so tenants at different points migrate independently, which
// one committed file shared by every tenant could not express.
//
// Scheduler run-state is a fifth (BUG-TKL08E): this build's job queue is
// durable and shared by every process on the database, so the runs it
// executes must be recorded where every one of those processes can see them.
func openBackend(
	ctx context.Context, base *SharedBase,
) (store.Store, search.Searcher, backendOverrides, io.Closer, error) {
	dsn := base.cfg.DatabaseURL
	if dsn == "" {
		return nil, nil, backendOverrides{}, nil, errors.New(
			"appbuild: postgres build requires a database URL (set RELA_DATABASE_URL)")
	}
	pool, poolCloser, err := pgstore.NewPool(ctx, dsn)
	if err != nil {
		return nil, nil, backendOverrides{}, nil, err
	}
	// Every failure below must close the pool: nothing else holds it yet, so
	// returning early without this leaks the connections.
	if err = pgstore.Migrate(ctx, pool); err != nil {
		_ = poolCloser.Close()
		return nil, nil, backendOverrides{}, nil, fmt.Errorf("migrate database: %w", err)
	}

	st, searcher, err := pgstore.Open(ctx, pool, dsn, pgstore.WithSearchTitles(searchTitles(base.meta)))
	if err != nil {
		_ = poolCloser.Close()
		return nil, nil, backendOverrides{}, nil, err
	}
	migState, err := pgmigstate.New(pool)
	if err != nil {
		_ = poolCloser.Close()
		return nil, nil, backendOverrides{}, nil, err
	}

	commentStore, err := pgcomments.New(pool)
	if err != nil {
		_ = poolCloser.Close()
		return nil, nil, backendOverrides{}, nil, err
	}
	schedState, err := pgschedstate.New(pool)
	if err != nil {
		_ = poolCloser.Close()
		return nil, nil, backendOverrides{}, nil, err
	}
	return st, searcher, backendOverrides{
		commentStore:   commentStore,
		migState:       migState,
		schedulerState: schedState,
	}, poolCloser, nil
}

// searchTitles derives the type-to-title-property map pgstore ranks free-text
// search by. It is built here because pgstore must not import the metamodel
// (a store does not depend on an application package); what crosses the
// boundary is plain strings, which pgstore binds as SQL parameters.
func searchTitles(meta *metamodel.Metamodel) pgstore.SearchTitles {
	titles := pgstore.SearchTitles{}
	for name := range meta.Entities {
		def := meta.Entities[name]
		if prop := metamodel.RankingTitleProperty(&def); prop != "" {
			titles[name] = prop
		}
	}
	return titles
}
