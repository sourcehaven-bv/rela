package acl

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// evaluatorReceivers are the types that evaluate an identity. They read the
// membership closure from the [Identity]; they never compute it.
var evaluatorReceivers = map[string]bool{"Declarative": true, "Request": true}

// identityCalls are the selectors that resolve an identity or walk
// membership. An evaluator method that calls one has gone back to resolving
// per evaluation, which is the cross-request coupling this split removed.
var identityCalls = map[string]bool{
	"walkMembers":                 true,
	"Resolve":                     true,
	"ResolveDirect":               true,
	"ResolvePrincipal":            true,
	"EffectiveMembershipRelation": true,
	"IdentityResolver":            true,
}

// membershipNames are the identifiers that name the membership relation. An
// OutgoingRelations call whose arguments mention one is the member-of walk
// written out by hand.
var membershipNames = map[string]bool{
	"EffectiveMembershipRelation": true,
	"MembershipRelation":          true,
	"defaultMembershipRelation":   true,
}

// identityAccessor is the one evaluator method allowed to touch the
// resolver: the accessor that hands it to the request edge.
const identityAccessor = "Declarative.IdentityResolver"

// identityCall is one forbidden call found in a function body.
type identityCall struct {
	pos    string
	callee string
}

// funcFacts is what the guard records about one function declaration: the
// identity calls it makes directly, and the functions and methods of the same
// package it may call.
type funcFacts struct {
	calls   []identityCall
	callees []string
}

// funcKey names a declaration: the bare name for a package-level function,
// Recv.Name for a method.
func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

// evaluatorIdentityCalls parses files as one package. It returns a finding
// for every identity call an evaluator method makes, directly or through a
// chain of functions and methods declared in the package.
//
// A method call is followed syntactically: x.name() may reach every method
// called name on any type in the package, since the scan does not know x's
// type. That over-approximates, so a finding through a method names the
// path it took. Methods of the evaluator types are not followed, because
// each is checked on its own.
func evaluatorIdentityCalls(fset *token.FileSet, files map[string][]byte) ([]string, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var decls []*ast.FuncDecl
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, files[name], 0)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
				decls = append(decls, fn)
			}
		}
	}

	scope := declScope{funcs: map[string]bool{}, methods: map[string][]string{}}
	for _, fn := range decls {
		key := funcKey(fn)
		if fn.Recv == nil {
			scope.funcs[key] = true
			continue
		}
		recv := receiverName(fn.Recv.List[0].Type)
		if evaluatorReceivers[recv] || key == identityAccessor {
			continue
		}
		scope.methods[fn.Name.Name] = append(scope.methods[fn.Name.Name], key)
	}
	facts := map[string]funcFacts{}
	for _, fn := range decls {
		facts[funcKey(fn)] = scanFunc(fset, fn.Body, scope)
	}

	var findings []string
	for _, fn := range decls {
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		recv := receiverName(fn.Recv.List[0].Type)
		method := funcKey(fn)
		if !evaluatorReceivers[recv] || method == identityAccessor {
			continue
		}
		own := facts[method]
		for _, c := range own.calls {
			findings = append(findings, c.pos+": "+method+" calls "+c.callee)
		}
		findings = append(findings, reachableFindings(method, own.callees, facts)...)
	}
	return findings, nil
}

// declScope is what a call may reach inside the package: its package-level
// functions, and its methods by name (minus the evaluator types' own).
type declScope struct {
	funcs   map[string]bool
	methods map[string][]string
}

// scanFunc records the identity calls body makes and the functions and
// methods of the package it may call.
func scanFunc(fset *token.FileSet, body *ast.BlockStmt, scope declScope) funcFacts {
	var out funcFacts
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var callee string
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			callee = fun.Sel.Name
			if !identityCalls[callee] {
				out.callees = append(out.callees, scope.methods[callee]...)
			}
		case *ast.Ident:
			callee = fun.Name
			if scope.funcs[callee] {
				out.callees = append(out.callees, callee)
			}
		}
		pos := fset.Position(call.Pos()).String()
		switch {
		case identityCalls[callee]:
			out.calls = append(out.calls, identityCall{pos, callee})
		case callee == "OutgoingRelations" && mentionsMembership(call.Args):
			out.calls = append(out.calls, identityCall{pos, "OutgoingRelations with the membership relation"})
		}
		return true
	})
	return out
}

// mentionsMembership reports whether any argument names the membership
// relation, by identifier or as the literal default.
func mentionsMembership(args []ast.Expr) bool {
	found := false
	for _, a := range args {
		ast.Inspect(a, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				found = found || membershipNames[x.Name]
			case *ast.BasicLit:
				found = found || x.Value == `"`+defaultMembershipRelation+`"`
			}
			return !found
		})
	}
	return found
}

// reachableFindings walks the package-level functions an evaluator method
// calls, transitively, and reports the identity calls they make on its behalf.
func reachableFindings(method string, roots []string, facts map[string]funcFacts) []string {
	type step struct{ name, path string }
	var out []string
	seen := map[string]bool{}
	queue := make([]step, 0, len(roots))
	for _, r := range roots {
		queue = append(queue, step{r, r})
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if seen[cur.name] {
			continue
		}
		seen[cur.name] = true
		f := facts[cur.name]
		for _, c := range f.calls {
			out = append(out, c.pos+": "+method+" reaches "+cur.path+", which calls "+c.callee)
		}
		for _, next := range f.callees {
			queue = append(queue, step{next, cur.path + " -> " + next})
		}
	}
	return out
}

func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// TestEvaluatorDoesNotResolveIdentity pins the split between resolving and
// evaluating. Resolution (the principal_property lookup and the member-of
// walk) runs once at the request edge in [IdentityResolver]. [Declarative]
// and [Request] only read the closure carried by an [Identity].
//
// If an evaluator method resolved again, the walk would run per evaluation
// instead of per request, and an evaluation path could silently pick a
// different identity than the one the edge stamped for audit. The scan
// covers every non-test file, so a new file cannot opt out, and it follows
// calls into package-level helpers and into methods of the package's other
// types, so neither can hide the walk.
func TestEvaluatorDoesNotResolveIdentity(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[name] = src
	}
	if len(files) == 0 {
		t.Fatal("scanned no files: the guard is vacuous")
	}
	findings, err := evaluatorIdentityCalls(token.NewFileSet(), files)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("%s\n\tResolve the identity at the request edge and read it from the Request.", f)
	}
}

// TestEvaluatorIdentityGuard_DetectsViolations proves the scan is not
// vacuous: each forbidden shape inside an evaluator method is reported,
// including one hidden two package-level helpers deep, one behind a method of
// another type, and a hand-written membership walk, and the same calls outside evaluator code are not.
func TestEvaluatorIdentityGuard_DetectsViolations(t *testing.T) {
	t.Parallel()
	const src = `package acl
func (r *Request) a() { r.d.ids.walkMembers(nil, "") }
func (d *Declarative) b() { d.IdentityResolver().ResolveDirect(nil, p) }
func (r Request) c() { _ = r.d.policy.EffectiveMembershipRelation() }
func (d *Declarative) IdentityResolver() *IdentityResolver { return d.ids }
func (r *IdentityResolver) d() { r.walkMembers(nil, "") }
func (r *Request) e() { helperOne(r) }
func helperOne(r *Request) { helperTwo(r) }
func helperTwo(r *Request) { r.d.ids.ResolveDirect(nil, p) }
func (r *Request) f() { r.d.graph.OutgoingRelations(nil, "u", "member-of") }
func (r *Request) g() { r.d.graph.OutgoingRelations(nil, "u", r.d.policy.MembershipRelation) }
func (r *Request) h() { r.d.graph.OutgoingRelations(nil, "u", "owns") }
func (r *Request) i() { r.view.closure() }
func (v *policyView) closure() { v.ids.walkMembers(nil, "") }
func unreached() { walkMembers(nil, "") }
`
	findings, err := evaluatorIdentityCalls(token.NewFileSet(), map[string][]byte{"fixture.go": []byte(src)})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Request.a calls walkMembers",
		"Declarative.b calls ResolveDirect",
		"Declarative.b calls IdentityResolver",
		"Request.c calls EffectiveMembershipRelation",
		"Request.e reaches helperOne -> helperTwo, which calls ResolveDirect",
		"Request.f calls OutgoingRelations with the membership relation",
		"Request.g calls OutgoingRelations with the membership relation",
		"Request.i reaches policyView.closure, which calls walkMembers",
	}
	if len(findings) != len(want) {
		t.Fatalf("findings = %v, want %d", findings, len(want))
	}
	for i, w := range want {
		if !strings.HasSuffix(findings[i], w) {
			t.Errorf("finding %d = %q, want suffix %q", i, findings[i], w)
		}
	}
}
