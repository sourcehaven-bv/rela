package dataentry

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestNoInternalErrorDetail is the guard for GitHub #1774: no 500 in this
// package may answer with an error's text. A store, file or database error
// can name paths, tables or rows the caller may not read, so the cause goes
// to the server log through [writeInternalError] instead.
//
// It looks at every call and composite literal that names
// http.StatusInternalServerError. Every other argument must be a literal, an
// http.* constant, or one of the few identifiers in safeIdents. A detail
// built from a variable fails, whatever the variable is called; the helpers
// themselves are exempt because their detail is the caller's, and their
// callers pass the error as a separate argument.
func TestNoInternalErrorDetail(t *testing.T) {
	exemptFuncs := map[string]bool{
		"writeInternalErrorDetail": true, // the one place a 500 detail is a parameter
		"writeInternalJSONError":   true, // appends internalErrorDetail to a caller literal
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && exemptFuncs[fn.Name.Name] {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				var args []ast.Expr
				switch v := n.(type) {
				case *ast.CallExpr:
					args = v.Args
				case *ast.CompositeLit:
					args = v.Elts
				default:
					return true
				}
				if !names500(args) {
					return true
				}
				for _, arg := range args {
					if !safe500Arg(arg) {
						t.Errorf("%s: a 500 takes a non-literal argument; use writeInternalError",
							fset.Position(arg.Pos()))
					}
				}
				return true
			})
		}
	}
}

// safeIdents are the identifiers a 500 call may pass: the response, the
// request, and the action correlation id, which the server generated.
var safeIdents = map[string]bool{
	"w": true, "r": true, "correlationID": true, "internalErrorDetail": true,
}

func names500(args []ast.Expr) bool {
	for _, arg := range args {
		if kv, ok := arg.(*ast.KeyValueExpr); ok {
			arg = kv.Value
		}
		if sel, ok := arg.(*ast.SelectorExpr); ok && sel.Sel.Name == "StatusInternalServerError" {
			return true
		}
	}
	return false
}

func safe500Arg(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return safeIdents[v.Name] || v.Name == "nil" || v.Name == "true" || v.Name == "false"
	case *ast.SelectorExpr:
		x, ok := v.X.(*ast.Ident)
		return ok && x.Name == "http"
	case *ast.KeyValueExpr:
		return safe500Arg(v.Value)
	case *ast.UnaryExpr:
		return safe500Arg(v.X)
	case *ast.CompositeLit:
		for _, elt := range v.Elts {
			if !safe500Arg(elt) {
				return false
			}
		}
		return true
	}
	return false
}

// TestWriteInternalError_HidesTheCause pins the helpers' contract: the
// client sees the code and title, the server log sees the cause, and a
// request the client abandoned gets neither.
func TestWriteInternalError_HidesTheCause(t *testing.T) {
	const secret = "pq: relation tenant_42.entities does not exist"
	for name, write := range map[string]func(http.ResponseWriter, *http.Request, error){
		"v1": func(w http.ResponseWriter, r *http.Request, err error) {
			writeInternalError(w, r, "read_failed", "Failed to read", err)
		},
		"json": func(w http.ResponseWriter, r *http.Request, err error) {
			writeInternalJSONError(w, r, "failed to save settings", err)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			rec := httptest.NewRecorder()
			write(rec, httptest.NewRequest(http.MethodGet, "/api/v1/x", http.NoBody), errors.New(secret))
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want 500", rec.Code)
			}
			body := rec.Body.String()
			if strings.Contains(body, "tenant_42") || !strings.Contains(body, internalErrorDetail) {
				t.Errorf("body = %s, want the generic detail and no cause", body)
			}
			if !strings.Contains(logs.String(), "tenant_42") {
				t.Errorf("log = %q, want the cause", logs.String())
			}

			logs.Reset()
			rec = httptest.NewRecorder()
			write(rec, httptest.NewRequest(http.MethodGet, "/api/v1/x", http.NoBody),
				errors.Join(errors.New(secret), context.Canceled))
			if rec.Body.Len() != 0 || logs.Len() != 0 {
				t.Errorf("canceled request: body %q, log %q; want neither", rec.Body.String(), logs.String())
			}
		})
	}
}

// TestCloneEntity_UniqueCollisionIs422 pins that a clone failing validation
// is the client's problem, not the server's. A `unique:` property always
// collides with the source it was copied from, so the response must say so
// rather than hide it behind the generic 500.
func TestCloneEntity_UniqueCollisionIs422(t *testing.T) {
	meta := testMeta()
	ticket := meta.Entities["ticket"]
	props := maps.Clone(ticket.Properties)
	props["code"] = metamodel.PropertyDef{Type: "string", Unique: true}
	ticket.Properties = props
	meta.Entities["ticket"] = ticket

	f := newFixture()
	f.AddNode(&entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "One", "code": "A"}})
	app := newAppFromParts(nil, meta, f)

	rec := httptest.NewRecorder()
	app.write.handleV1CloneEntity(rec,
		httptest.NewRequest(http.MethodPost, "/api/v1/tickets/TKT-001/_actions/clone", http.NoBody),
		"ticket", "TKT-001")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "validation_failed") {
		t.Fatalf("clone = %d %s, want 422 validation_failed", rec.Code, rec.Body.String())
	}
}
