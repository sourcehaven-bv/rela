package entitymanager_test

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

// readGates are the calls that refuse a row the caller cannot read as a
// missing one. A write to an existing row runs one of them before it asks
// the ACL, so a 403 never confirms the row exists (see the entitymanager
// CLAUDE.md, "PatchEntity ordering is load-bearing").
var readGates = map[string]bool{
	"requireReadableRow": true,
	"visibleEndpoint":    true,
	"lookupSource":       true,
	"endpointVisible":    true,
}

// gateWrappers are functions that count as a read gate because they call one
// of readGates themselves. The scan checks that they still do, so a wrapper
// that lost its gate fails here instead of passing every caller.
var gateWrappers = map[string]string{
	"readTaskItemSource": "TaskItemConverter's source read: requireReadableRow on the family, " +
		"then the face gate on the named row",
}

// gateFamilyArg is the argument index of each gate's row family. A gate
// called with a nil or empty literal there gates nothing: requireReadableRow
// passes an empty family, and visibleEndpoint finds the zero family
// unreadable without asking the policy about any row.
var gateFamilyArg = map[string]int{
	"requireReadableRow": 3,
	"visibleEndpoint":    2,
}

// authorizers are the calls that ask the ACL about a write.
var authorizers = []string{"authorizeAndAudit", "AuthorizeWrite"}

// callerGates lists helpers that ask the ACL, directly or through another
// listed helper, and rely on their caller to have run the read gate. Each
// reason names that caller. The scan treats a call to a listed helper like a
// call to the ACL, so every caller is checked in turn and a helper on this
// list cannot hide an ungated caller.
var callerGates = map[string]string{
	"authorizeAndAudit": "the authorizer itself: it calls ACL.AuthorizeWrite",
	"authorizeTag":      "VersionTags.prepare runs requireReadableRow on the row before calling it",
	"authorizeRename": "RenameEntity runs requireReadableRow on the family before it calls " +
		"familyRename.inTx, which authorizes that family and its renamed copy",
	"inTx": "familyRename.inTx: RenameEntity gates the family before the transaction",
	"authorizeFamily": "DeleteEntity and SoftDeleteEntity gate the family first; owned deletes " +
		"reach rows through a gated owner, and restores leave the gate to RestoreEntity's caller",
	"authorizeCascadeRelations": "authorizes the edges a gated delete destroys; the deleted row was " +
		"gated by the delete's entry point",
	"deleteEntityInTx":    "DeleteEntity runs requireReadableRow on the family before the transaction",
	"softDeleteInTx":      "SoftDeleteEntity runs requireReadableRow on the family before the transaction",
	"prepareOwnedDeletes": "owned rows are reached through the owner, which the delete gated",
	"deleteOwned":         "owned rows are reached through the owner, which the delete gated",
	"authorizeRestore":    "restoreInTx calls it; RestoreEntity leaves the read gate to its caller",
	"restoreInTx":         "RestoreEntity's critical section; RestoreEntity leaves the read gate to its caller",
	"authorizeCopy":       "planCopy gates the source and the target before calling it",
	"deletePrepass": "guardFamilyDelete and checkSoftDelete call it; DeleteEntity and " +
		"SoftDeleteEntity run requireReadableRow on the family before them",
	"checkFaceDeleteEdges": "guardFaceDelete and the face delete's txEdges call it; the face " +
		"delete runs requireReadableRow on the face before guardFaceDelete",
	"guardFamilyDelete": "DeleteEntity runs requireReadableRow on the family before it",
	"checkSoftDelete":   "SoftDeleteEntity runs requireReadableRow on the family before it",
	"guardFaceDelete":   "the face delete runs requireReadableRow on the face before it",
	"txEdges":           "faceDeleteGuard.txEdges: the face delete gated the face before the transaction",
	"prepareRelationFrom": "prepareRelationCreate runs lookupSource before it; CreateLinked passes the " +
		"source it is creating",
	"authorizeTaskItem": "plan gates the source through readTaskItemSource before it, and " +
		"TaskItemConverter.Offers probes an entity the request already read",
}

// noReadGateNeeded lists the functions that ask the ACL with no read gate
// before it and whose callers need none either, each with the reason. A call
// to one of these does not count as asking the ACL. Adding a name is a
// deliberate exception.
var noReadGateNeeded = map[string]string{
	"prepareCreate": "CreateEntity's checks: creates a row; no stored row exists for a 403 to confirm",
	"CheckCreate":   "probes CreateEntity's type-level create question; it names no stored row",
	"prepareLinkedRelation": "CreateLinked's edge checks: one end of the edge is the row being created, so " +
		"no stored row exists for a 403 to confirm; the existing end is gated by lookupSource, or by " +
		"prepareRelationFrom for a target",
	"recreateEntity": "authorizes a create of a face that is not stored, as CreateEntity does",
	"RestoreEntity": "documented to leave the read gate to its caller (the data-entry restore " +
		"handler); a marked row is hidden from every read, so the store cannot gate it here",
	"planCopy": "readCopySource gates the source (requireReadableSource) and probeCopyTarget gates the " +
		"target (PermitsReadFace) before authorizeCopy; both go through the face gate the scan does not follow",
	"edgeTypeAllowed": "an affordance probe for the copy's own edges: withAffordanceProbe drops the " +
		"audit row and the copy skips the edge instead of answering 403",
	"Offers": "an affordance probe on a served entity: data-entry calls Converter.Offers from " +
		"computeConversionOffers and TaskItemConverter.Offers from computeTaskItemOffers, both for an " +
		"entity the request already read, and withAffordanceProbe keeps a denial out of the audit log",
	"authorizeRemovals": "conversionRun.apply calls it after Converter.plan, which runs requireReadableRow " +
		"on the source family; the removed edges are that family's, judged by edgeVisibility",
}

// TestAuthorizeAndAuditFollowsReadGate fails when a function in this
// package asks the ACL, or calls a function on noReadGateNeeded, without
// first calling a read gate, unless the function is itself on the list. A
// new write path that asks the ACL first would answer 403 for a hidden row
// and 404 for a missing one. It also fails on a read gate called with an
// empty family, which gates nothing.
func TestAuthorizeAndAuditFollowsReadGate(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	res := scanReadGates(fset, files, callerGates, noReadGateNeeded)
	for _, b := range res.bad {
		t.Errorf("%s: no read gate before it; run requireReadableRow (or visibleEndpoint, "+
			"endpointVisible, lookupSource) first, or add the function to callerGates or noReadGateNeeded with a reason", b)
	}
	for _, e := range res.emptyGates {
		t.Errorf("%s: read gate called with an empty family, which gates no row", e)
	}
	for fn := range gateWrappers {
		if !res.gated[fn] {
			t.Errorf("%s is listed in gateWrappers but no longer calls a read gate", fn)
		}
	}
	for _, list := range []map[string]string{callerGates, noReadGateNeeded} {
		for fn := range list {
			if fn != "authorizeAndAudit" && !res.ungated[fn] {
				t.Errorf("%s is allowlisted but no longer asks the ACL ungated; remove it", fn)
			}
		}
	}
}

// TestAuthorizeAndAuditFollowsReadGate_Planted pins that the scan catches a
// violation and accepts a gated call, so the guard cannot pass vacuously.
func TestAuthorizeAndAuditFollowsReadGate_Planted(t *testing.T) {
	t.Parallel()
	const src = `package entitymanager

func (m *Manager) Gated(ctx context.Context, row *entity.Entity) error {
	if err := requireReadableRow(ctx, faceGateOf(m), row.ID, family); err != nil {
		return err
	}
	return m.authorizeAndAudit(ctx, req)
}

func (m *Manager) Planted(ctx context.Context) error {
	if err := m.authorizeAndAudit(ctx, req); err != nil {
		return err
	}
	return requireReadableRow(ctx, faceGateOf(m), "X", family)
}

func (m *Manager) Closure(ctx context.Context) error {
	return m.store.Tx(ctx, func(ctx context.Context) error {
		return m.authorizeAndAudit(ctx, req)
	})
}

func (m *Manager) Direct(ctx context.Context) error {
	return m.deps.ACL.AuthorizeWrite(ctx, req)
}

func (m *Manager) EmptyGate(ctx context.Context) error {
	if err := requireReadableRow(ctx, faceGateOf(m), "X", nil); err != nil {
		return err
	}
	if _, err := visibleEndpoint(ctx, faceGateOf(m), entityFamily{}, ""); err != nil {
		return err
	}
	return m.authorizeAndAudit(ctx, req)
}

func (m *Manager) helper(ctx context.Context) error {
	return m.authorizeAndAudit(ctx, req)
}

func (m *Manager) HelperCaller(ctx context.Context) error {
	return m.helper(ctx)
}

func (m *Manager) GatedHelperCaller(ctx context.Context) error {
	if _, err := lookupSource(ctx, m, "X", ""); err != nil {
		return err
	}
	return m.helper(ctx)
}

func (m *Manager) terminal(ctx context.Context) error {
	return m.authorizeAndAudit(ctx, req)
}

func (m *Manager) TerminalCaller(ctx context.Context) error {
	return m.terminal(ctx)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	res := scanReadGates(fset, []*ast.File{f},
		map[string]string{"authorizeAndAudit": "planted", "helper": "planted"},
		map[string]string{"terminal": "planted"})
	var got []string
	for _, b := range res.bad {
		got = append(got, b[strings.Index(b, "(")+1:len(b)-1])
	}
	if want := "Planted,Closure,Direct,EmptyGate,HelperCaller"; strings.Join(got, ",") != want {
		t.Errorf("ungated = %v, want %s: the gate after the call must not count, a call inside a "+
			"closure belongs to its enclosing function, a direct AuthorizeWrite counts, an empty-family "+
			"gate does not gate, and a caller of a listed helper is checked too", got, want)
	}
	if len(res.emptyGates) != 2 {
		t.Errorf("empty gates = %v, want the nil family and the zero entityFamily", res.emptyGates)
	}
}

// readGateScan is the result of [scanReadGates].
type readGateScan struct {
	bad        []string        // ungated calls in functions not on the allowlist
	emptyGates []string        // gate calls with an empty family literal
	ungated    map[string]bool // functions that make an ungated call
	gated      map[string]bool // functions that call one of readGates
}

// gateFacts is what the scan records about one top-level function.
type gateFacts struct {
	name      string
	firstGate token.Pos
	calls     []namedCall
}

type namedCall struct {
	callee string
	pos    token.Pos
}

// scanReadGates checks every top-level function in files. A call to an
// authorizer, or to an ungated function on callers, needs a read gate before
// it in the same function, else the function must be on callers or terminal.
// A function on terminal ends the chain. Closures count as part of the
// function that contains them.
func scanReadGates(
	fset *token.FileSet, files []*ast.File, callers, terminal map[string]string,
) readGateScan {
	res := readGateScan{ungated: map[string]bool{}, gated: map[string]bool{}}
	var funcs []gateFacts
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			facts := gateFacts{name: fn.Name.Name}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := calleeName(call)
				if readGates[name] || gateWrappers[name] != "" {
					if emptyFamily(call, name) {
						res.emptyGates = append(res.emptyGates,
							fset.Position(call.Pos()).String()+" ("+fn.Name.Name+")")
					} else if !facts.firstGate.IsValid() || call.Pos() < facts.firstGate {
						facts.firstGate = call.Pos()
					}
					return true
				}
				facts.calls = append(facts.calls, namedCall{name, call.Pos()})
				return true
			})
			if facts.firstGate.IsValid() {
				res.gated[fn.Name.Name] = true
			}
			funcs = append(funcs, facts)
		}
	}

	// The set of guarded callees grows with every allowlisted function that
	// is found ungated, until no new one turns up.
	guarded := map[string]bool{}
	for _, a := range authorizers {
		guarded[a] = true
	}
	reported := map[token.Pos]bool{}
	for changed := true; changed; {
		changed = false
		for _, fn := range funcs {
			for _, c := range fn.calls {
				if !guarded[c.callee] || c.callee == fn.name {
					continue
				}
				if fn.firstGate.IsValid() && fn.firstGate < c.pos {
					continue
				}
				res.ungated[fn.name] = true
				if _, ok := terminal[fn.name]; ok {
					continue
				}
				if _, ok := callers[fn.name]; ok {
					if !guarded[fn.name] {
						guarded[fn.name] = true
						changed = true
					}
					continue
				}
				if !reported[c.pos] {
					reported[c.pos] = true
					res.bad = append(res.bad, fset.Position(c.pos).String()+" ("+fn.name+")")
				}
			}
		}
	}
	sort.SliceStable(res.bad, func(i, j int) bool { return lessPos(res.bad[i], res.bad[j]) })
	return res
}

// emptyFamily reports whether the gate call passes nil or an empty composite
// literal as its row family.
func emptyFamily(call *ast.CallExpr, gate string) bool {
	i, ok := gateFamilyArg[gate]
	if !ok || i >= len(call.Args) {
		return false
	}
	switch arg := call.Args[i].(type) {
	case *ast.Ident:
		return arg.Name == "nil"
	case *ast.CompositeLit:
		return len(arg.Elts) == 0
	}
	return false
}

// lessPos orders "file:line:col (fn)" strings by file, then line and column.
func lessPos(a, b string) bool {
	pa, pb := splitPos(a), splitPos(b)
	if pa.file != pb.file {
		return pa.file < pb.file
	}
	if pa.line != pb.line {
		return pa.line < pb.line
	}
	return pa.col < pb.col
}

type posKey struct {
	file      string
	line, col int
}

func splitPos(s string) posKey {
	s, _, _ = strings.Cut(s, " ")
	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return posKey{file: s}
	}
	var k posKey
	k.file = strings.Join(parts[:len(parts)-2], ":")
	k.line = atoi(parts[len(parts)-2])
	k.col = atoi(parts[len(parts)-1])
	return k
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
