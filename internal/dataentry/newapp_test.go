package dataentry

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// loadOnlyLoader is a config.Loader with none of the optional capabilities
// (Stat, Dirs, Open) that serving custom/ and apps/ needs.
type loadOnlyLoader struct{ config.Loader }

// stubTemplater is a non-nil templating.Templater; NewApp only checks it is
// present before failing on the loader.
type stubTemplater struct{ templating.Templater }

// NewApp rejects a missing project-files loader, a missing templater and a
// loader that cannot serve custom/ and apps/ before it touches any of them.
// Data-entry config, scripts and assets are all read through that one loader
// (FEAT-UP14BT), so a nil one must fail at construction, not on the first
// request.
func TestNewApp_RejectsMissingConfigCollaborators(t *testing.T) {
	tests := []struct {
		name      string
		files     config.Loader
		templater templating.Templater
		wantErr   string
	}{
		{name: "nil files loader", files: nil, templater: stubTemplater{}, wantErr: "files is required"},
		{name: "nil templater", files: loadOnlyLoader{}, templater: nil, wantErr: "templater is required"},
		{name: "loader without Stat and Dirs", files: loadOnlyLoader{}, templater: stubTemplater{},
			wantErr: "must support Stat and Dirs"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, err := NewApp(nil, nil, tc.files, tc.templater, nil, nil, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil)
			if err == nil {
				t.Fatalf("NewApp succeeded (app = %v), want an error containing %q", app, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("NewApp err = %q, want it to contain %q", err, tc.wantErr)
			}
			if app != nil {
				t.Fatal("NewApp returned an app alongside its error")
			}
		})
	}
}

// NewApp refuses a nil ACL even when every other collaborator is real. The
// ACL type switches in the handlers have no nil arm to fall back on, and a
// missing ACL must not read as "no restrictions".
func TestNewApp_RejectsNilACL(t *testing.T) {
	root := t.TempDir()
	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}
	fsys := storage.NewSafeFS(storage.NewOsFS())
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"ticket": {Label: "Ticket", IDPrefix: "TKT-", Properties: map[string]metamodel.PropertyDef{}},
	}}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fsys, paths))
	t.Cleanup(func() { _ = svc.Close() })

	app, err := NewApp(
		svc.FS(), svc.Paths(), svc.ProjectFiles(), svc.Templater(), svc.Meta(), svc.Store(), stubVersionService{},
		svc.EntityManager(), svc.Searcher(), svc.VisibleSearcher(), nil,
		NopFieldVerdictResolver{}, svc.Audit(), svc.State(), UngatedCommandAuthorizer(), appbuild.CompiledWorlds(svc),
	)
	if err == nil || !strings.Contains(err.Error(), "acl is required") {
		t.Fatalf("NewApp with a nil ACL: err = %v, want it to contain %q", err, "acl is required")
	}
	if app != nil {
		t.Fatal("NewApp returned an app alongside its error")
	}
}
