package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// newNoIdentityApp returns the v1 API routes of an app whose read gate,
// affordances and entitymanager write path all evaluate one Declarative
// policy, without the attachACLRequest edge in front. Each test decides
// what the edge would have attached.
func newNoIdentityApp(t *testing.T) (http.Handler, *acl.Declarative) {
	t.Helper()
	meta := newTestAppV1(t).Meta()
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
	if err := bootstrap.Store().CreateEntity(context.Background(),
		&entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	all := []string{"ticket", "feature"}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: all, Create: all, Update: all, Delete: all},
		},
		Assignments: map[string]string{"alice": "editor", "bob": "editor"},
	}, bootstrap.Store())
	svc := appbuildtest.New(meta,
		appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(bootstrap.Store()),
		appbuildtest.WithDeclarative(d),
	)
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, d)
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	mux := http.NewServeMux()
	app.registerAPIV1Routes(mux)
	return mux, d
}

// TestNoIdentity_Status pins the status a data-entry request gets when it
// reaches a handler without a usable identity (BUG-O51NS5). Production
// /api routes always pass attachACLRequest, which binds one or fails the
// request, so this is the fail-closed answer for a route or a ctx that lost
// it:
//
//   - no Request bound (acl.ErrNoIdentity) is 401, never 500 or 403;
//   - a Request bound to another principal (acl.ErrIdentityMismatch) is a
//     server fault and stays 500;
//   - with no read gate either, reads still answer the uniform 404 of a
//     hidden entity, for an existing and a missing id alike.
func TestNoIdentity_Status(t *testing.T) {
	t.Parallel()

	// Each mode builds the ctx the edge would have handed the handler.
	type ctxMode func(t *testing.T, d *acl.Declarative) context.Context
	alice := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	bob := principal.Principal{User: "bob", Tool: principal.ToolDataEntry}
	// The read gate is attached, so reads run; the Request is missing, so
	// every RequestFrom below the edge fails with acl.ErrNoIdentity.
	gateWithoutRequest := func(t *testing.T, d *acl.Declarative) context.Context {
		t.Helper()
		r := acltest.RequestFor(t.Context(), t, d, alice)
		return withReadGate(withACLEnforced(principal.With(t.Context(), alice)), aclReadGate{req: r})
	}
	// A Request for bob on a ctx stamped for alice.
	mismatched := func(t *testing.T, d *acl.Declarative) context.Context {
		t.Helper()
		r := acltest.RequestFor(t.Context(), t, d, bob)
		ctx := acl.WithRequest(principal.With(t.Context(), alice), r)
		return withReadGate(withACLEnforced(ctx), aclReadGate{req: r})
	}
	// Neither gate nor Request: reads get the deny gate.
	nothingBound := func(t *testing.T, _ *acl.Declarative) context.Context {
		t.Helper()
		return withACLEnforced(principal.With(t.Context(), alice))
	}

	cases := []struct {
		name   string
		mode   ctxMode
		method string
		path   string
		body   string
		want   int
	}{
		{"read/no request", gateWithoutRequest, http.MethodGet, "/api/v1/tickets/TKT-001", "", http.StatusOK},
		{"list/no request", gateWithoutRequest, http.MethodGet, "/api/v1/tickets", "", http.StatusOK},
		{"create/no request", gateWithoutRequest, http.MethodPost, "/api/v1/tickets",
			`{"properties":{"title":"x"}}`, http.StatusUnauthorized},
		{"update/no request", gateWithoutRequest, http.MethodPatch, "/api/v1/tickets/TKT-001",
			`{"properties":{"title":"y"}}`, http.StatusUnauthorized},
		{"delete/no request", gateWithoutRequest, http.MethodDelete, "/api/v1/tickets/TKT-001", "",
			http.StatusUnauthorized},

		{"create/mismatch", mismatched, http.MethodPost, "/api/v1/tickets",
			`{"properties":{"title":"x"}}`, http.StatusInternalServerError},
		{"update/mismatch", mismatched, http.MethodPatch, "/api/v1/tickets/TKT-001",
			`{"properties":{"title":"y"}}`, http.StatusInternalServerError},
		{"delete/mismatch", mismatched, http.MethodDelete, "/api/v1/tickets/TKT-001", "",
			http.StatusInternalServerError},

		{"read/nothing bound", nothingBound, http.MethodGet, "/api/v1/tickets/TKT-001", "", http.StatusNotFound},
		{"read missing/nothing bound", nothingBound, http.MethodGet, "/api/v1/tickets/TKT-404", "",
			http.StatusNotFound},
		{"update/nothing bound", nothingBound, http.MethodPatch, "/api/v1/tickets/TKT-001",
			`{"properties":{"title":"y"}}`, http.StatusNotFound},
		{"update missing/nothing bound", nothingBound, http.MethodPatch, "/api/v1/tickets/TKT-404",
			`{"properties":{"title":"y"}}`, http.StatusNotFound},
		{"delete/nothing bound", nothingBound, http.MethodDelete, "/api/v1/tickets/TKT-001", "",
			http.StatusNotFound},
		{"delete missing/nothing bound", nothingBound, http.MethodDelete, "/api/v1/tickets/TKT-404", "",
			http.StatusNotFound},
		{"create/nothing bound", nothingBound, http.MethodPost, "/api/v1/tickets",
			`{"properties":{"title":"x"}}`, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, d := newNoIdentityApp(t)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(tc.mode(t, d))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: got %d, want %d; body=%s", tc.method, tc.path, rec.Code, tc.want, rec.Body)
			}
			if tc.want == http.StatusUnauthorized &&
				!strings.Contains(rec.Body.String(), "/errors/unauthenticated") {

				t.Errorf("401 body is not the unauthenticated problem: %s", rec.Body)
			}
		})
	}
}

// TestNoIdentity_SharedMappers pins that every shared error mapper answers a
// wrapped acl.ErrNoIdentity with 401 and leaves an identity mismatch at 500,
// so a handler cannot get it wrong by choosing a mapper.
func TestNoIdentity_SharedMappers(t *testing.T) {
	t.Parallel()
	// leakMarker stands for internal detail in the wrapped error text, such
	// as a table name. It must never reach a response body.
	const leakMarker = "secret_table_name"
	noIdentity := fmt.Errorf("read gate %s: %w", leakMarker, acl.ErrNoIdentity)
	mismatch := fmt.Errorf("read gate %s: %w", leakMarker, acl.ErrIdentityMismatch)
	forbidden := func(cause error) error {
		return fmt.Errorf("update %s: %w", leakMarker, &acl.ForbiddenError{Decision: acl.Decision{
			RuleKind: "role-grant", RuleID: "-", Reason: "acl: no request scope", Cause: cause,
		}})
	}
	type errCase struct {
		name string
		err  error
		want int
	}
	plainErrs := []errCase{
		{"no identity", noIdentity, http.StatusUnauthorized},
		{"mismatch", mismatch, http.StatusInternalServerError},
	}
	// The deny mappers judge Decision.Cause only: a rule deny joined with an
	// unrelated ErrNoIdentity stays a 403.
	denyErrs := []errCase{
		{"no identity", forbidden(acl.ErrNoIdentity), http.StatusUnauthorized},
		{"mismatch", forbidden(acl.ErrIdentityMismatch), http.StatusInternalServerError},
		{"foreign", forbidden(acl.ErrForeignIdentity), http.StatusInternalServerError},
		{"rule deny", forbidden(nil), http.StatusForbidden},
		{"rule deny joined", errors.Join(forbidden(nil), acl.ErrNoIdentity), http.StatusForbidden},
	}
	mappers := []struct {
		name  string
		errs  []errCase
		write func(w http.ResponseWriter, r *http.Request, err error)
	}{
		{"writeGateError", plainErrs, writeGateError},
		{"writeInternalError", plainErrs, func(w http.ResponseWriter, r *http.Request, err error) {
			writeInternalError(w, r, "update_failed", "Failed to update entity", err)
		}},
		{"writeInternalJSONError", plainErrs, func(w http.ResponseWriter, r *http.Request, err error) {
			writeInternalJSONError(w, r, "Failed to save settings", err)
		}},
		{"writeForbiddenIfACLDenied", denyErrs, func(w http.ResponseWriter, r *http.Request, err error) {
			if !writeForbiddenIfACLDenied(w, r, err) {
				t.Errorf("writeForbiddenIfACLDenied did not handle %v", err)
			}
		}},
		{"writeConversionError", denyErrs, writeConversionError},
		{"writeCopyError", denyErrs, writeCopyError},
		{"failACLDenied", denyErrs, func(w http.ResponseWriter, r *http.Request, err error) {
			fe, ok := errors.AsType[*acl.ForbiddenError](err)
			if !ok {
				t.Fatalf("not a ForbiddenError: %v", err)
			}
			failACLDenied(fe).writeHTTP(w, r)
		}},
	}
	for _, m := range mappers {
		errs := m.errs
		for _, e := range errs {
			t.Run(m.name+"/"+e.name, func(t *testing.T) {
				t.Parallel()
				req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001", http.NoBody)
				rec := httptest.NewRecorder()
				m.write(rec, req, e.err)
				if rec.Code != e.want {
					t.Fatalf("got %d, want %d; body=%s", rec.Code, e.want, rec.Body)
				}
				if strings.Contains(rec.Body.String(), leakMarker) {
					t.Errorf("body echoes the error text: %s", rec.Body)
				}
			})
		}
	}
	if !errors.Is(forbidden(acl.ErrNoIdentity), acl.ErrNoIdentity) {
		t.Fatal("ForbiddenError does not unwrap to Decision.Cause")
	}
}
