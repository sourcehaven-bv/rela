package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/configedit"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// validateConfigEditing refuses --config-editing where it would be unsafe or
// cannot work. envUser is $RELA_DATAENTRY_USER, passed in like
// validateIdentityFlags does.
func validateConfigEditing(f *serverFlags, svc *appbuild.Services, mode identityMode, envUser string) error {
	switch {
	case !configEditingBackend:
		// The sqlite store holds an exclusive lock a rebuild cannot take
		// twice, and several postgres nodes would each switch on their own.
		return errors.New("only the filesystem build supports it")
	case f.readOnly:
		return errors.New("cannot be combined with --read-only")
	case svc.ACLDeclarative() == nil:
		// Without acl.yaml every principal holds every permission.
		return errors.New("needs an acl.yaml that grants config:edit")
	case mode != identityJWT && f.principalHeader == "" && envUser == "":
		return errors.New("needs an identity source (-jwt-*, -principal-header or $" +
			dataentry.EnvDataEntryUserVar + ")")
	case svc.Paths().SchemaIsLegacy:
		return fmt.Errorf("the schema file is %s; rename it to %s first",
			svc.Paths().SchemaPath, configedit.SchemaFile)
	}
	return nil
}

// newConfigure builds the Configure API for s. It is built once, so its save
// mutex serializes saves across every rebuild of the app.
func newConfigure(s *server, svc *appbuild.Services) (http.Handler, error) {
	if err := validateConfigEditing(s.f, svc, s.mode, os.Getenv(dataentry.EnvDataEntryUserVar)); err != nil {
		return nil, err
	}
	files, err := storage.NewRootedFS(svc.FS(), svc.Paths().Root)
	if err != nil {
		return nil, err
	}
	h := configureHost{s: s}
	cs, err := configedit.NewService(configedit.Deps{
		Files:      files,
		Validator:  h,
		Counter:    h,
		Migrations: h,
		Activator:  h,
		Audit:      h,
	})
	if err != nil {
		return nil, err
	}
	slog.Warn("configuration editing is on: principals holding config:edit can change " +
		"schema.yaml and data-entry.yaml and run data migrations from the browser")
	return cs.Handler(dataentry.ConfigurePath), nil
}

// configureHost gives the Configure API what it needs from the running
// server. Every read goes to the generation serving at that moment.
type configureHost struct{ s *server }

func (h configureHost) current() *generation { return h.s.cur.Load() }

// Record writes to the serving generation's audit log.
func (h configureHost) Record(rec audit.Record) { h.current().svc.Audit().Record(rec) }

// Validate checks a schema and data-entry pair the way `rela validate` does,
// and the loaded acl.yaml against the new schema. Building the candidate
// server in Prepare is the complete check; this one runs on every preview.
func (h configureHost) Validate(schema, dataEntry []byte) (*metamodel.Metamodel, error) {
	meta, err := metamodel.Parse(schema)
	if err != nil {
		return nil, &configedit.ValidationError{Problems: []string{"schema.yaml: " + err.Error()}}
	}
	var problems []string
	for _, p := range appbuild.CheckSchemaCompiles(meta, h.current().svc.Store()) {
		problems = append(problems, "schema.yaml: "+p)
	}
	if err := projectsetup.CheckDataEntry(dataEntry, meta); err != nil {
		var cve *dataentry.ConfigValidationError
		if errors.As(err, &cve) {
			for _, e := range cve.Errors {
				problems = append(problems, "data-entry.yaml: "+e)
			}
		} else {
			problems = append(problems, "data-entry.yaml: "+err.Error())
		}
	}
	if policy := h.current().svc.ACLPolicy(); policy != nil {
		if err := appbuild.ValidateACLPolicy(policy, meta); err != nil {
			problems = append(problems, "acl.yaml: "+err.Error())
		}
	}
	if len(problems) > 0 {
		return nil, &configedit.ValidationError{Problems: problems}
	}
	return meta, nil
}

// Count counts the records of entityType whose property holds value, or any
// value when value is empty. It reads the raw store: the Configure API only
// serves config:edit holders, and returns counts, never records. It counts
// every face row, because a migration rewrites every one of them.
func (h configureHost) Count(ctx context.Context, entityType, property, value string) (int, error) {
	pred := store.PropPredicate{Property: property, Op: store.PropEqual, Value: value}
	if value == "" {
		// An empty Value with PropNotEqual means "is not empty".
		pred.Op = store.PropNotEqual
	}
	st := h.current().svc.Store()
	return store.CountMatched(ctx, st, store.GraphQuery{
		EntityType: entityType, Faces: store.AllFaces(), Props: []store.PropPredicate{pred},
	})
}

// Ready reports whether the store's migration record allows a new migration.
func (h configureHost) Ready(ctx context.Context, current *metamodel.Metamodel) error {
	_, err := readyGate(ctx, h.current().svc, current)
	return err
}

// Adopt is Ready that also records a compatible change to current, as
// `rela migrate` would, so the save's migration file starts from the
// recorded shape.
func (h configureHost) Adopt(ctx context.Context, current *metamodel.Metamodel) error {
	svc := h.current().svc
	gate, err := readyGate(ctx, svc, current)
	if err != nil {
		return err
	}
	if _, err = gate.EvaluateAndPersist(ctx, current); err != nil {
		return err
	}
	// Persist skips rather than waits when a migration or cleanup run holds
	// the lock, so check that the record actually moved.
	v, err := gate.Evaluate(ctx, current)
	if err != nil {
		return err
	}
	if v.Status != datamigration.StatusInSync {
		return errors.New("could not record the current data shape; a data migration may be running, try again")
	}
	return nil
}

// Pause holds the serving generation still; see server.pause.
func (h configureHost) Pause(ctx context.Context) (func(), error) { return h.s.pause(ctx, h.current()) }

// Prepare builds a complete server over the files on disk. Failures are the
// files' fault as far as the user can act on them, so they come back as
// problems.
//
//nolint:contextcheck // appbuild.Discover and dataentry.NewApp take no context to pass
func (h configureHost) Prepare(context.Context) (configedit.Candidate, error) {
	svc, err := discoverProject(h.s.f, appbuild.WithInMemorySearch())
	if err != nil {
		slog.Warn("configedit: the new configuration does not load", "error", err)
		return nil, &configedit.ValidationError{Problems: []string{err.Error()}}
	}
	if svc.ACLDeclarative() == nil {
		closeServices(svc)
		return nil, &configedit.ValidationError{Problems: []string{"acl.yaml no longer loads"}}
	}
	g, err := h.s.build(svc)
	if err != nil {
		closeServices(svc)
		var cve *dataentry.ConfigValidationError
		if errors.As(err, &cve) {
			return nil, &configedit.ValidationError{Problems: cve.Errors}
		}
		slog.Warn("configedit: the server could not be built from the new configuration", "error", err)
		return nil, &configedit.ValidationError{Problems: []string{err.Error()}}
	}
	return &candidate{s: h.s, g: g}, nil
}

// candidate is a generation built by Prepare, not yet serving.
type candidate struct {
	s *server
	g *generation
}

// Migrate runs the pending migrations against the candidate's services.
func (c *candidate) Migrate(ctx context.Context) (int, error) {
	svc := c.g.svc
	plan, fsys, err := pendingPlan(ctx, svc)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", configedit.ErrMigrationNotStarted, err) //nolint:errorlint // one %w is the category
	}
	var capture datamigration.VersionCapture
	if v := svc.Versions(); v != nil {
		capture = v
	}
	lock := datamigration.LockFor(svc.Store(), svc.Paths().CacheDir)
	runner, err := datamigration.NewRunner(datamigration.Deps{
		Store:    svc.Store(),
		Meta:     svc.Meta(),
		State:    svc.State(),
		MigState: svc.MigState(),
		Audit:    svc.Audit(),
		ScriptFS: fsys,
		Versions: capture,
		Lock:     lock,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %v", configedit.ErrMigrationNotStarted, err) //nolint:errorlint // one %w is the category
	}
	res, err := runner.Run(ctx, plan, true)
	changed := 0
	if res != nil {
		for _, f := range res.Files {
			for _, st := range f.Steps {
				changed += st.Affected
			}
		}
	}
	switch {
	case err == nil:
	case errors.Is(err, datamigration.ErrLockHeld):
		return 0, err
	case res == nil || len(res.Files) == 0:
		// Refused before the first step (the lock, the count before), so
		// no record changed and the old files can still be put back.
		return 0, fmt.Errorf("%w: %v", configedit.ErrMigrationNotStarted, err) //nolint:errorlint // one %w is the category
	default:
		// Every file may have run with only the count after failing; then
		// the migration is done and only the report is missing.
		if left, _, pErr := pendingPlan(ctx, svc); pErr != nil || len(left) > 0 {
			return changed, err
		}
		slog.Warn("configedit: migration applied; counting records afterwards failed", "error", err)
	}
	// Let the gate adopt any compatible remainder, as `rela migrate data` does.
	if gate, gErr := newGate(svc); gErr == nil {
		if _, gErr = gate.EvaluateAndPersist(ctx, svc.Meta()); gErr != nil {
			slog.Warn("configedit: recording the data shape after the migration", "error", gErr)
		}
	}
	// The candidate's own gate classified the data before it was migrated;
	// its GC sweep acts on that verdict, so refresh it before serving.
	if gErr := appbuild.ReevaluateDataGate(ctx, svc); gErr != nil {
		slog.Warn("configedit: re-checking the data shape after the migration", "error", gErr)
	}
	return changed, nil
}

// Activate switches the server to the candidate.
func (c *candidate) Activate() { c.s.activate(c.g) }

// Discard releases a candidate that will not serve.
func (c *candidate) Discard() {
	c.g.app.StopWatching()
	closeServices(c.g.svc)
}

func closeServices(svc *appbuild.Services) {
	if err := svc.Close(); err != nil {
		slog.Warn("closing unused services", "error", err)
	}
}

// configFS views svc's project configuration as an fs.FS bound to ctx.
func configFS(ctx context.Context, svc *appbuild.Services) (fs.FS, error) {
	v, err := config.NewFSView(svc.Config())
	if err != nil {
		return nil, err
	}
	return v.WithContext(ctx)
}

// newGate builds the migration gate over svc's record, the same way
// appbuild and the CLI do.
func newGate(svc *appbuild.Services) (*datamigration.Gate, error) {
	return datamigration.NewGate(datamigration.GateDeps{
		MigState: svc.MigState(),
		State:    svc.State(),
		Lock:     datamigration.LockFor(svc.Store(), svc.Paths().CacheDir),
		HasMigrations: func(ctx context.Context) (bool, error) {
			fsys, err := configFS(ctx, svc)
			if err != nil {
				return false, err
			}
			return datamigration.HasMigrations(fsys)
		},
	})
}

// readyGate returns svc's gate when its record allows a new migration from
// current: the stored records conform to current, or differ only
// compatibly, and every migration file has run.
func readyGate(ctx context.Context, svc *appbuild.Services, current *metamodel.Metamodel) (*datamigration.Gate, error) {
	gate, err := newGate(svc)
	if err != nil {
		return nil, err
	}
	v, err := gate.Evaluate(ctx, current)
	if err != nil {
		return nil, err
	}
	switch v.Status {
	case datamigration.StatusInSync, datamigration.StatusAdopted, datamigration.StatusBootstrapped:
	default:
		return nil, fmt.Errorf("the stored records are not ready for a new migration: %s "+
			"Run `rela migrate status` on the server", v.Describe())
	}
	fsys, err := configFS(ctx, svc)
	if err != nil {
		return nil, err
	}
	files, err := datamigration.LoadDir(fsys)
	if err != nil {
		return nil, err
	}
	var applied []string
	if st, err := svc.MigState().Load(ctx); err != nil {
		return nil, err
	} else if st != nil {
		applied = st.AppliedNames()
	}
	for _, f := range files {
		if !slices.Contains(applied, f.Name) {
			return nil, fmt.Errorf("migration %s has not run yet; run `rela migrate data` on the server first", f.Name)
		}
	}
	return gate, nil
}

// pendingPlan resolves the migrations svc's store still needs, from its
// recorded shape to svc's schema.
func pendingPlan(ctx context.Context, svc *appbuild.Services) ([]*datamigration.File, fs.FS, error) {
	fsys, err := configFS(ctx, svc)
	if err != nil {
		return nil, nil, err
	}
	files, err := datamigration.LoadDir(fsys)
	if err != nil {
		return nil, nil, err
	}
	st, err := svc.MigState().Load(ctx)
	if err != nil {
		return nil, nil, err
	}
	if st == nil {
		return nil, nil, errors.New("the store has no migration record")
	}
	stored, err := st.ShapeProjection()
	if err != nil {
		return nil, nil, err
	}
	plan, err := datamigration.Resolve(stored, st.AppliedNames(), svc.Meta().ShapeProjection(), files)
	if err != nil {
		return nil, nil, err
	}
	return plan, fsys, nil
}
