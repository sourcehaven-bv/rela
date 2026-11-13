package archguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

// storeTree is exempt from the unpaged-read guard: the store package, its
// backends and its reference implementations ARE the reads the guard counts
// for everyone else.
const storeTree = "internal/store"

// unpagedReadGuard is the unpaged-read rule's failure vocabulary.
var unpagedReadGuard = guard{
	what: "unpaged collection read",
	list: "unpagedReadAllowlist",
	advice: "a request must not read a whole type or the whole graph, because its cost then grows with " +
		"the store, not with the page it serves (.claude/rules/collection-reads.md). Page it in the store " +
		"(ListEntitiesPage or GraphQuery with Limit), push the filter into a GraphQuery, or bound it by " +
		"ids (EntityQuery.IDs, RelationQuery.EntityIDs). If reading everything IS the job (a migration, " +
		"an import, a whole-graph report), add an entry with category batchJob and say what the job is. " +
		"If you only moved an existing call between functions, move its allowlist entry with it",
}

// readCategory says why an unpaged read may stay.
type readCategory string

const (
	// batchJob: reading the whole collection is what the operation is for: a
	// migration, an import or export, a whole-graph analysis, a reindex, a
	// CLI command that dumps every row. A whole-graph report served on
	// request (/_analyze, an MCP analyze tool) is a batch job too: its
	// output is about every row, so no page could serve it.
	batchJob readCategory = "batch-job"
	// requestPath: a handler, MCP tool, script binding or read path serving
	// one request whose answer is a page or a few rows, yet whose cost grows
	// with the store. This is debt; the follow-ups of TKT-FED6FMM remove it.
	requestPath readCategory = "request-path"
	// bounded: the call is unpaged, but its filter bounds the result
	// independently of the store's size (a lookup on a unique property, or
	// a type that holds a handful of configuration rows). The reason must
	// name the bound.
	bounded readCategory = "bounded"
	// passThrough: a reader's internal helper that runs the query its
	// caller built, but changes it first, so the matcher does not treat it
	// as a query helper. The reason names who calls it.
	passThrough readCategory = "pass-through"
)

// unpagedRead is one function's pinned count of unpaged reads, with why they
// may stay.
type unpagedRead struct {
	n        int
	category readCategory
	reason   string
}

// alwaysUnpaged are the read methods that never page: the store ignores
// Cursor and Limit on them. ListEntityHeaders is both the store.HeaderReader
// method and the store.ListEntityHeaders function.
var alwaysUnpaged = map[string]bool{
	"ListEntities":      true,
	"ListRelations":     true,
	"ListEntityHeaders": true,
}

// pageable are the read methods that page only when the query sets Limit;
// Limit 0 returns every match in one go.
var pageable = map[string]bool{
	"ListEntitiesPage":  true,
	"ListRelationsPage": true,
	"GraphQuery":        true,
	"GraphQueryHeaders": true,
}

// keyFields are the query fields that bound a read by ids: EntityQuery.IDs,
// and RelationQuery's endpoint filters. A query that sets one reads the rows
// of those ids, however large the store is.
var keyFields = map[string]bool{
	"IDs":       true,
	"EntityIDs": true,
	"EntityID":  true,
	"From":      true,
	"To":        true,
	"PeerIDs":   true,
}

// queryHelper is a function that hands one of its parameters to a read as
// the query (a reader decorator, or a helper such as a drain loop). Its read
// is charged to its callers, which are matched like a read themselves.
type queryHelper struct {
	param    int  // index of the query parameter
	pageable bool // the read it wraps pages when the query sets Limit
}

// unpagedReads returns every unpaged whole-collection read in the given
// files of ONE package, keyed by position, with the enclosing function
// ("Func" or "Recv.Method") as the value.
//
// A call is an unpaged whole-collection read when all of these hold:
//
//   - It calls ListEntities, ListRelations or ListEntityHeaders, on any
//     receiver (an interface method call counts) or as store.ListEntityHeaders.
//     These never page. Or it calls ListEntitiesPage, ListRelationsPage,
//     GraphQuery or GraphQueryHeaders with a query that sets no Limit. Or it
//     calls a query helper of the same package (see below), with a query
//     that would make the helper's read unpaged.
//   - Its query sets no id key (IDs, EntityIDs, EntityID, From, To, PeerIDs).
//     Such a read is a batched lookup: it grows with the ids asked for, not
//     with the store. A GraphQuery has no id key; its predicates may still
//     bound it, which the allowlist records as category bounded.
//   - Its query is not a parameter of the enclosing function that the
//     function never reassigns. Such a function is a query helper: a reader
//     decorator's ListEntities, or a drain loop such as collectRelations.
//     Its read is charged to its callers, where the query is built. Callers
//     of a helper named like a read method are matched in every package;
//     callers of any other helper only in its own package.
//
// The query is the read's query argument: the last one, or the helper's
// query parameter. The match reads it syntactically: a composite literal's
// keys, or, for a local variable, the assignments to that name in the
// statements before the call in each block that encloses it, in order. An
// assignment nested in another block (an if branch) is ignored, so a key set
// on one path only does not exempt the call. A key set to a literal zero
// (nil, 0, "") counts as unset. A query the matcher cannot see (a struct
// field, a call result, a variable a helper fills through a pointer) counts
// as unpaged.
//
// Not caught: a key or Limit that is zero at run time (EntityQuery.IDs empty
// means every id; RelationQuery.From "" means any source; Limit 0 means
// all), a method value (`r.ListEntities` passed as a func), a read in a
// package-level function literal, an exported helper called from another
// package, counts (CountEntities, GraphCount), and whole-graph reads through
// other services (tracer, search). Review covers those.
func unpagedReads(fset *token.FileSet, files []*ast.File) map[token.Position]string {
	helpers := queryHelpers(files)
	found := map[token.Position]string{}
	for _, file := range files {
		for _, fn := range funcDecls(file) {
			name := funcName(fn)
			forEachRead(fn, helpers, func(call *ast.CallExpr, query ast.Expr, pageable bool) {
				if _, isParam := unmodifiedParam(fn, query); isParam {
					return
				}
				fields := queryFields(fn.Body, call, query)
				if hasKey(fields) || (pageable && fields["Limit"]) {
					return
				}
				found[fset.Position(call.Fun.Pos())] = name
			})
		}
	}
	return found
}

// queryHelpers finds the package's query helpers, to a fixed point: a
// function that forwards its parameter to a helper is a helper too.
func queryHelpers(files []*ast.File) map[string]queryHelper {
	helpers := map[string]queryHelper{}
	for changed := true; changed; {
		changed = false
		for _, file := range files {
			for _, fn := range funcDecls(file) {
				if _, known := helpers[fn.Name.Name]; known {
					continue
				}
				forEachRead(fn, helpers, func(_ *ast.CallExpr, query ast.Expr, pageable bool) {
					if i, ok := unmodifiedParam(fn, query); ok {
						helpers[fn.Name.Name] = queryHelper{param: i, pageable: pageable}
						changed = true
					}
				})
			}
		}
	}
	return helpers
}

func funcDecls(file *ast.File) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			out = append(out, fn)
		}
	}
	return out
}

// forEachRead calls visit for every read call in fn's body: a guarded read
// method, or a call of one of helpers, with the call's query argument.
func forEachRead(
	fn *ast.FuncDecl, helpers map[string]queryHelper, visit func(*ast.CallExpr, ast.Expr, bool),
) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		var name string
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = f.Sel.Name
			if alwaysUnpaged[name] || pageable[name] {
				visit(call, call.Args[len(call.Args)-1], pageable[name])
				return true
			}
		case *ast.Ident:
			name = f.Name
		default:
			return true
		}
		if h, ok := helpers[name]; ok && h.param < len(call.Args) {
			visit(call, call.Args[h.param], h.pageable)
		}
		return true
	})
}

// unmodifiedParam returns the index of the parameter query names, when it is
// one that fn never assigns to, assigns a field of, or takes the address of.
func unmodifiedParam(fn *ast.FuncDecl, query ast.Expr) (int, bool) {
	id, ok := unwrap(query).(*ast.Ident)
	if !ok {
		return 0, false
	}
	index, found := 0, -1
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			index++
			continue
		}
		for _, name := range field.Names {
			if name.Name == id.Name {
				found = index
			}
			index++
		}
	}
	if found < 0 {
		return 0, false
	}
	modified := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if rootIdent(lhs) == id.Name {
					modified = true
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && rootIdent(n.X) == id.Name {
				modified = true
			}
		}
		return !modified
	})
	return found, !modified
}

// rootIdent returns the name e assigns through: x for x, x.F and x.F.G.
func rootIdent(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x.Name
		case *ast.SelectorExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return ""
		}
	}
}

// funcName is fn's allowlist name: "Func", or "Recv.Method" for a method.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	switch r := t.(type) {
	case *ast.IndexExpr: // generic receiver T[P]
		t = r.X
	case *ast.IndexListExpr:
		t = r.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func hasKey(fields map[string]bool) bool {
	for f, set := range fields {
		if set && keyFields[f] {
			return true
		}
	}
	return false
}

// queryFields returns the fields query sets when call runs, as far as the
// statements before call in its enclosing blocks show them.
func queryFields(body *ast.BlockStmt, call *ast.CallExpr, query ast.Expr) map[string]bool {
	query = unwrap(query)
	if lit, ok := query.(*ast.CompositeLit); ok {
		return litKeys(lit)
	}
	id, ok := query.(*ast.Ident)
	if !ok {
		return nil
	}
	fields := map[string]bool{}
	for _, stmts := range enclosingStmts(body, call) {
		for _, stmt := range stmts {
			if stmt.End() > call.Pos() {
				break // the statement holding the call, and everything after it
			}
			applyStmt(fields, stmt, id.Name)
		}
	}
	return fields
}

// enclosingStmts returns the statement lists of every block on the path
// from body down to call, outermost first.
func enclosingStmts(body *ast.BlockStmt, call *ast.CallExpr) [][]ast.Stmt {
	var out [][]ast.Stmt
	walk := func(n ast.Node) bool {
		if n == nil || n.Pos() > call.Pos() || n.End() < call.End() {
			return false
		}
		switch b := n.(type) {
		case *ast.BlockStmt:
			out = append(out, b.List)
		case *ast.CaseClause:
			out = append(out, b.Body)
		case *ast.CommClause:
			out = append(out, b.Body)
		}
		return true
	}
	ast.Inspect(body, walk)
	return out
}

// applyStmt updates fields with what stmt sets on the variable name.
func applyStmt(fields map[string]bool, stmt ast.Stmt, name string) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		for i, lhs := range s.Lhs {
			if i >= len(s.Rhs) {
				break
			}
			switch l := lhs.(type) {
			case *ast.Ident:
				if l.Name == name {
					replaceFields(fields, s.Rhs[i])
				}
			case *ast.SelectorExpr:
				if x, ok := l.X.(*ast.Ident); ok && x.Name == name {
					fields[l.Sel.Name] = !isZeroLit(s.Rhs[i])
				}
			}
		}
	case *ast.DeclStmt:
		gen, ok := s.Decl.(*ast.GenDecl)
		if !ok {
			return
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, nm := range vs.Names {
				if nm.Name != name {
					continue
				}
				var value ast.Expr
				if i < len(vs.Values) {
					value = vs.Values[i]
				}
				replaceFields(fields, value)
			}
		}
	}
}

// replaceFields resets fields to what value, assigned whole, sets.
func replaceFields(fields map[string]bool, value ast.Expr) {
	clear(fields)
	if lit, ok := unwrap(value).(*ast.CompositeLit); ok {
		maps.Copy(fields, litKeys(lit))
	}
}

// unwrap strips parentheses and a leading & from e.
func unwrap(e ast.Expr) ast.Expr {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return e
			}
			e = x.X
		default:
			return e
		}
	}
}

// litKeys returns the fields lit sets to something other than a literal zero.
func litKeys(lit *ast.CompositeLit) map[string]bool {
	keys := map[string]bool{}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if k, ok := kv.Key.(*ast.Ident); ok && !isZeroLit(kv.Value) {
				keys[k.Name] = true
			}
		}
	}
	return keys
}

// isZeroLit reports whether e is nil, 0 or an empty string literal.
func isZeroLit(e ast.Expr) bool {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name == "nil"
	}
	lit, ok := e.(*ast.BasicLit)
	return ok && (lit.Value == "0" || isEmptyString(lit))
}

// scanUnpagedReads returns the unpaged reads under root, keyed by
// "repo/relative/file.go#Func", outside the store tree. Files are grouped by
// directory, which is the package, so a helper's callers are found.
func scanUnpagedReads(t *testing.T, root string) map[string][]token.Position {
	t.Helper()
	var fset *token.FileSet
	pkgs := map[string][]*ast.File{}
	collect := func(fs *token.FileSet, file *ast.File) []token.Position {
		fset = fs
		dir := filepath.Dir(fs.Position(file.Pos()).Filename)
		pkgs[dir] = append(pkgs[dir], file)
		return nil
	}
	scanTree(t, root, scannedRoots, collect)
	got := map[string][]token.Position{}
	for _, files := range pkgs {
		for pos, fn := range unpagedReads(fset, files) {
			rel, err := filepath.Rel(root, pos.Filename)
			if err != nil {
				t.Fatal(err)
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, storeTree+"/") {
				continue
			}
			key := rel + "#" + fn
			got[key] = append(got[key], pos)
		}
	}
	return got
}

// TestNoNewUnpagedReads pins every unpaged whole-collection read in non-test
// code outside the store to unpagedReadAllowlist, exactly, per function.
func TestNoNewUnpagedReads(t *testing.T) {
	t.Parallel()
	got := scanUnpagedReads(t, repoRoot)
	want := make(map[string]int, len(unpagedReadAllowlist))
	for key, e := range unpagedReadAllowlist {
		want[key] = e.n
	}
	for _, msg := range diffAllowlist(unpagedReadGuard, got, want) {
		t.Error(msg)
	}
}

func TestUnpagedReadAllowlist_HasReasons(t *testing.T) {
	t.Parallel()
	for key, e := range unpagedReadAllowlist {
		if strings.TrimSpace(e.reason) == "" {
			t.Errorf("%s: allowlist entry has no reason", key)
		}
		if e.n <= 0 {
			t.Errorf("%s: allowlist count %d; delete the entry instead", key, e.n)
		}
		switch e.category {
		case batchJob, requestPath, bounded, passThrough:
		default:
			t.Errorf("%s: unknown category %q", key, e.category)
		}
		if !strings.Contains(key, ".go#") {
			t.Errorf("%s: key must be file.go#Func", key)
		}
	}
}

// The request-path scan TKT-A6A97WV's budget tests name stays on the list
// as debt, not relabelled. TKT-D3RAB0E deletes this test with the scan.
func TestUnpagedReadAllowlist_ScopedHeadersIsDebt(t *testing.T) {
	t.Parallel()
	const key = "internal/dataentry/scopedread.go#scopedHeaders"
	e, ok := unpagedReadAllowlist[key]
	if !ok {
		t.Fatalf("%s: not on the allowlist; if TKT-D3RAB0E removed the scan, delete this test with it", key)
	}
	if e.category != requestPath {
		t.Errorf("%s: category %q, want %q", key, e.category, requestPath)
	}
}

func TestUnpagedReads(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want map[string]int // function -> findings
	}{
		{"type scan", `func f() { st.ListEntities(ctx, store.EntityQuery{Type: "t"}) }`, map[string]int{"f": 1}},
		{"relation scan", `func f() { st.ListRelations(ctx, store.RelationQuery{Type: "r"}) }`, map[string]int{"f": 1}},
		{"header function", `func f() { store.ListEntityHeaders(ctx, st, store.EntityQuery{}) }`, map[string]int{"f": 1}},
		{"by ids", `func f() { st.ListEntities(ctx, store.EntityQuery{IDs: ids}) }`, nil},
		{"by endpoint", `func f() { st.ListRelations(ctx, store.RelationQuery{From: id}) }`, nil},
		{"by endpoints", `func f() { st.ListRelations(ctx, store.RelationQuery{EntityIDs: ids}) }`, nil},
		{"pointer literal", `func f() { st.ListRelations(ctx, &store.RelationQuery{To: id}) }`, nil},
		{"variable with key", `func f() { q := store.EntityQuery{Type: "t"}; q.IDs = ids; st.ListEntities(ctx, q) }`, nil},
		{"variable literal key", `func f() { var q = store.RelationQuery{EntityID: id}; st.ListRelations(ctx, q) }`, nil},
		{"variable without key", `func f() { q := store.EntityQuery{Type: "t"}; st.ListEntities(ctx, q) }`,
			map[string]int{"f": 1}},
		{"parameter is a helper", `func f(q store.EntityQuery) { st.ListEntities(ctx, q) }`, nil},
		{"page without limit", `func f() { st.ListEntitiesPage(ctx, store.EntityQuery{Type: "t"}) }`,
			map[string]int{"f": 1}},
		{"page with limit", `func f() { st.ListEntitiesPage(ctx, store.EntityQuery{Type: "t", Limit: 50}) }`, nil},
		{"graph query without limit", `func f() { store.GraphQueryHeaders(ctx, st, store.GraphQuery{EntityType: "t"}) }`,
			map[string]int{"f": 1}},
		{"graph query with limit", `func f() { q := store.GraphQuery{}; q.Limit = 10; st.GraphQuery(ctx, q) }`, nil},
		{"graph query literal is no call", `func f() { _ = store.GraphQuery{EntityType: "t"} }`, nil},
		{"forwarding decorator",
			`func (r *R) ListEntities(ctx context.Context, q store.EntityQuery) { r.inner.ListEntities(ctx, q) }`, nil},
		{"decorator widening its query",
			`func (r *R) ListEntities(ctx context.Context, q store.EntityQuery) { q.IDs = nil; r.inner.ListEntities(ctx, q) }`,
			map[string]int{"R.ListEntities": 1}},
		{"read method with its own query",
			`func (r *R) ListRelationsStrict(ctx context.Context, q store.RelationQuery) { r.inner.ListRelations(ctx, store.RelationQuery{}) }`,
			map[string]int{"R.ListRelationsStrict": 1}},
		{"helper charged to a whole-type caller",
			`func (r *R) load(ctx context.Context, q store.EntityQuery) { r.inner.ListEntities(ctx, q) }
func g(r *R) { r.load(ctx, store.EntityQuery{Type: "t"}) }`, map[string]int{"g": 1}},
		{"helper with a keyed caller",
			`func drain(ctx context.Context, st S, q store.RelationQuery) { st.ListRelations(ctx, q) }
func g() { drain(ctx, st, store.RelationQuery{EntityID: id}) }`, nil},
		{"helper of a helper",
			`func drain(ctx context.Context, st S, q store.RelationQuery) { st.ListRelations(ctx, q) }
func outer(q store.RelationQuery) { drain(ctx, st, q) }
func g() { outer(store.RelationQuery{}) }`, map[string]int{"g": 1}},
		{"pageable helper with limit",
			`func run(q store.GraphQuery) { store.GraphQueryHeaders(ctx, st, q) }
func g() { run(store.GraphQuery{Limit: 10}) }`, nil},
		{"key set on one branch only",
			`func f() { q := store.RelationQuery{Type: t}; if id != "" { q.From = id }; st.ListRelations(ctx, q) }`,
			map[string]int{"f": 1}},
		{"key set after the call",
			`func f() { q := store.EntityQuery{}; st.ListEntities(ctx, q); q.IDs = ids }`, map[string]int{"f": 1}},
		{"key reset to nil", `func f() { q := store.EntityQuery{IDs: ids}; q.IDs = nil; st.ListEntities(ctx, q) }`,
			map[string]int{"f": 1}},
		{"variable reassigned without key",
			`func f() { q := store.EntityQuery{IDs: ids}; q = store.EntityQuery{}; st.ListEntities(ctx, q) }`,
			map[string]int{"f": 1}},
		{"key inside the call's own block",
			`func f() { for _, id := range ids { q := store.RelationQuery{}; q.From = id; st.ListRelations(ctx, q) } }`, nil},
		{"literal zero limit", `func f() { st.ListEntitiesPage(ctx, store.EntityQuery{Limit: 0}) }`, map[string]int{"f": 1}},
		{"literal empty key", `func f() { st.ListRelations(ctx, store.RelationQuery{From: ""}) }`, map[string]int{"f": 1}},
		{"method form graph query", `func f() { st.GraphQueryHeaders(ctx, store.GraphQuery{}) }`, map[string]int{"f": 1}},
		{"page limit on a variable",
			`func f() { var q store.RelationQuery; q.Limit = n; st.ListRelationsPage(ctx, q) }`, nil},
		{"method", `func (r *R) load() { r.st.ListRelations(ctx, store.RelationQuery{}) }`, map[string]int{"R.load": 1}},
		{"generic method", `func (r R[T]) load() { r.st.ListRelations(ctx, store.RelationQuery{}) }`,
			map[string]int{"R.load": 1}},
		{"closure", `func f() { g := func() { st.ListEntities(ctx, store.EntityQuery{}) }; g() }`, map[string]int{"f": 1}},
		{"other method", `func f() { st.ListFamilyAttachments(ctx, id) }`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "x.go", "package p\n"+tc.src+"\n", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := map[string]int{}
			for _, fn := range unpagedReads(fset, []*ast.File{file}) {
				got[fn]++
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("findings = %v, want %v", got, tc.want)
			}
		})
	}
}

// The guard must fail on a synthetic violation end to end, key it by
// function, and skip tests and the store tree.
func TestScanUnpagedReads_FindsSyntheticRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const bad = "func handle() { st.ListEntities(ctx, store.EntityQuery{Type: \"t\"}) }\n"
	for rel, pkg := range map[string]string{
		"internal/pkg/bad.go":                 "pkg",
		"internal/pkg/bad_test.go":            "pkg",
		"internal/store/fsstore/x.go":         "fsstore",
		"internal/store/graphquerynaive/n.go": "graphquerynaive",
		"internal/store/store.go":             "store",
		"cmd/tool/main.go":                    "main",
	} {
		if err := mkdirAllWrite(filepath.Join(root, filepath.FromSlash(rel)), "package "+pkg+"\n"+bad); err != nil {
			t.Fatal(err)
		}
	}
	got := scanUnpagedReads(t, root)
	if len(got) != 2 || len(got["internal/pkg/bad.go#handle"]) != 1 || len(got["cmd/tool/main.go#handle"]) != 1 {
		t.Fatalf("scan = %v, want one finding in each of internal/pkg/bad.go#handle and cmd/tool/main.go#handle", got)
	}
	msgs := diffAllowlist(unpagedReadGuard, got, map[string]int{"cmd/tool/main.go#handle": 1})
	if len(msgs) != 1 || !strings.HasPrefix(msgs[0], "internal/pkg/bad.go#handle:") {
		t.Errorf("want one finding for the unlisted function; got %q", msgs)
	}
}
