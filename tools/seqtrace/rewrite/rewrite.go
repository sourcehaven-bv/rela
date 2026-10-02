// Package rewrite injects seqtrace calls into Go source.
//
// It edits text at node offsets rather than reprinting the AST, and every
// insertion stays on the line where it starts. Comments, compiler directives
// and line numbers survive unchanged, so a panic in an instrumented binary
// still points at the right line.
//
// Three rewrites, matching the ways the runtime finds a parent:
//
//	func (s *S) M(ctx context.Context) {        // declared function
//	    f := rt.Enter(ctx, "pkg", "(*S).M"); defer f.Exit(); ctx = f.Ctx(ctx)
//
//	func(x int) error { ... }                   // function literal
//	func() func(x int) error { c := rt.Current(); return func(x int) error {
//	    f := rt.EnterClosure(nil, c, false, ...); defer f.Exit(); ... } }()
//
//	go s.sweep(ctx)                             // go statement, non-literal
//	rt.Spawn("sweep"); go s.sweep(ctx)
//
// A function literal that is the operand of a go statement gets the literal
// rewrite with goStmt set, so it always adopts its creator.
package rewrite

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// Runtime is the import path of the runtime package the injected code calls.
const Runtime = "github.com/Sourcehaven-BV/rela/tools/seqtrace"

const (
	rtName    = "seqtrace__rt"
	frameVar  = "seqtrace__f"
	creatorVr = "seqtrace__c"
	resultVar = "seqtrace__r"
)

// File returns src with tracing injected. pkg is the label recorded for every
// call in the file. changed is false when the file has nothing to instrument
// (or uses cgo), in which case out is nil and the original should be used.
func File(filename string, src []byte, pkg string) (out []byte, changed bool, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, false, err
	}
	r := &rewriter{fset: fset, file: fset.File(f.Pos()), src: src, pkg: pkg, ctxPkg: "context"}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		switch path {
		case "C":
			return nil, false, nil
		case "context":
			if imp.Name != nil {
				r.ctxPkg = imp.Name.Name
			}
		}
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body == nil || hasDirective(d.Doc) {
				continue
			}
			name := funcName(d)
			r.prologue(d.Body, fmt.Sprintf("%s.Enter(%s, %q, %q", rtName, r.ctxArg(d.Type), pkg, name), d.Type)
			r.walk(d.Body, name)
		case *ast.GenDecl:
			r.walk(d, "init")
		}
	}
	if len(r.ins) == 0 {
		return nil, false, nil
	}
	r.add(f.Name.End(), fmt.Sprintf("; import %s %q", rtName, Runtime))
	return r.apply(), true, nil
}

type insertion struct {
	off  int
	seq  int
	text string
	del  int // bytes of the original replaced by text, from off
}

type rewriter struct {
	fset   *token.FileSet
	file   *token.File
	src    []byte
	pkg    string
	ctxPkg string // local name of the "context" import
	ins    []insertion
}

func (r *rewriter) add(pos token.Pos, text string) {
	r.ins = append(r.ins, insertion{off: r.file.Offset(pos), seq: len(r.ins), text: text})
}

// replace swaps the source between pos and end for text.
func (r *rewriter) replace(pos, end token.Pos, text string) {
	off := r.file.Offset(pos)
	r.ins = append(r.ins, insertion{off: off, seq: len(r.ins), text: text, del: r.file.Offset(end) - off})
}

// prologue inserts the Enter/Exit pair (and the ctx reassignment) right
// after the body's opening brace. enter is the Enter call without its
// closing parenthesis; prologue appends the parameters to it.
//
// Exit must see the results as they are when the function returns, so it
// runs in a deferred closure over named results. Unnamed results are given
// names first, which does not change behavior: only a bare return can tell
// the difference, and a function with unnamed results cannot contain one.
func (r *rewriter) prologue(body *ast.BlockStmt, enter string, typ *ast.FuncType) {
	var args strings.Builder
	ctxName := r.ctxParam(typ)
	for _, field := range fieldsOf(typ.Params) {
		for _, n := range field.Names {
			if n.Name != "_" && n.Name != ctxName {
				fmt.Fprintf(&args, ", %q, %s", n.Name, wrapOpaque(field.Type, n.Name))
			}
		}
	}
	exit := fmt.Sprintf("defer %s.Exit();", frameVar)
	if results := r.nameResults(typ); len(results) > 0 {
		exit = fmt.Sprintf("defer func() { %s.Exit(%s) }();", frameVar, strings.Join(results, ", "))
	}
	text := fmt.Sprintf(" %s := %s%s); %s", frameVar, enter, args.String(), exit)
	if name := r.ctxParam(typ); name != "" {
		text += fmt.Sprintf(" %s = %s.Ctx(%s);", name, frameVar, name)
	}
	r.add(body.Lbrace+1, text)
}

// walk rewrites function literals and go statements under n. outer names
// the enclosing declaration, used to label literals.
func (r *rewriter) walk(n ast.Node, outer string) {
	count := 0
	goLits := map[*ast.FuncLit]bool{}
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			fun := x.Call.Fun
			for p, ok := fun.(*ast.ParenExpr); ok; p, ok = fun.(*ast.ParenExpr) {
				fun = p.X
			}
			if lit, ok := fun.(*ast.FuncLit); ok {
				goLits[lit] = true
			} else if name := calleeName(x.Call.Fun); name != "" {
				r.add(x.Pos(), fmt.Sprintf("%s.Spawn(%q); ", rtName, name))
			}
		case *ast.FuncLit:
			count++
			r.funcLit(x, fmt.Sprintf("%s.func%d", outer, count), goLits[x])
		}
		return true
	})
}

func (r *rewriter) funcLit(lit *ast.FuncLit, name string, goStmt bool) {
	if hasTag(lit.Type) {
		// typeText would reformat a struct tag, and the wrapper's type would
		// no longer match the literal's. Leave such literals alone.
		return
	}
	typ := r.typeText(lit.Type)
	r.add(lit.Pos(), fmt.Sprintf("func() %s { %s := %s.Current(); return ", typ, creatorVr, rtName))
	enter := fmt.Sprintf("%s.EnterClosure(%s, %s, %t, %q, %q",
		rtName, r.ctxArg(lit.Type), creatorVr, goStmt, r.pkg, name)
	r.prologue(lit.Body, enter, lit.Type)
	r.add(lit.End(), " }()")
}

// typeText prints a function type on one line, so copying it into the
// wrapper does not shift line numbers. Printing a bare node drops comments.
func (r *rewriter) typeText(t *ast.FuncType) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), t)
	return strings.Join(strings.Fields(buf.String()), " ")
}

// hasTag reports a struct tag anywhere in t.
func hasTag(t *ast.FuncType) bool {
	found := false
	ast.Inspect(t, func(n ast.Node) bool {
		if f, ok := n.(*ast.Field); ok && f.Tag != nil {
			found = true
		}
		return !found
	})
	return found
}

// nameResults gives every unnamed or blank result a name, by editing the
// signature, and returns the names of all results in order.
func (r *rewriter) nameResults(t *ast.FuncType) []string {
	if t.Results == nil || len(t.Results.List) == 0 {
		return nil
	}
	bare := !t.Results.Opening.IsValid() // `func f() error`: add parentheses
	if bare {
		r.add(t.Results.Pos(), "(") // before the name, which shares its offset
	}
	var names []string
	for _, field := range t.Results.List {
		if len(field.Names) == 0 {
			name := fmt.Sprintf("%s%d", resultVar, len(names))
			r.add(field.Type.Pos(), name+" ")
			names = append(names, wrapOpaque(field.Type, name))
			continue
		}
		for _, n := range field.Names {
			name := n.Name
			if name == "_" {
				name = fmt.Sprintf("%s%d", resultVar, len(names))
				r.replace(n.Pos(), n.End(), name)
			}
			names = append(names, wrapOpaque(field.Type, name))
		}
	}
	if bare {
		r.add(t.Results.End(), ")")
	}
	return names
}

// wrapOpaque wraps expr in the runtime's Opaque when typ is any or
// interface{}, so the runtime records only the dynamic type.
func wrapOpaque(typ ast.Expr, expr string) string {
	switch t := typ.(type) {
	case *ast.Ident:
		if t.Name != "any" {
			return expr
		}
	case *ast.InterfaceType:
		if t.Methods != nil && len(t.Methods.List) > 0 {
			return expr
		}
	default:
		return expr
	}
	return fmt.Sprintf("%s.Opaque(%s)", rtName, expr)
}

func fieldsOf(l *ast.FieldList) []*ast.Field {
	if l == nil {
		return nil
	}
	return l.List
}

// ctxParam returns the name of the first named context.Context parameter.
func (r *rewriter) ctxParam(t *ast.FuncType) string {
	if t.Params == nil {
		return ""
	}
	for _, field := range t.Params.List {
		sel, ok := field.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Context" {
			continue
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != r.ctxPkg {
			continue
		}
		for _, n := range field.Names {
			if n.Name != "_" {
				return n.Name
			}
		}
	}
	return ""
}

func (r *rewriter) ctxArg(t *ast.FuncType) string {
	if name := r.ctxParam(t); name != "" {
		return name
	}
	return "nil"
}

func (r *rewriter) apply() []byte {
	sort.SliceStable(r.ins, func(i, j int) bool {
		if r.ins[i].off != r.ins[j].off {
			return r.ins[i].off < r.ins[j].off
		}
		return r.ins[i].seq < r.ins[j].seq
	})
	var out bytes.Buffer
	out.Grow(len(r.src) + len(r.ins)*96)
	prev := 0
	for _, in := range r.ins {
		out.Write(r.src[prev:in.off])
		out.WriteString(in.text)
		prev = in.off + in.del
	}
	out.Write(r.src[prev:])
	return out.Bytes()
}

// funcName renders a declaration as the runtime labels it: "F", "T.M" or
// "(*T).M", with type parameters dropped.
func funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	t := d.Recv.List[0].Type
	ptr := false
	if s, ok := t.(*ast.StarExpr); ok {
		ptr, t = true, s.X
	}
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	recv := "?"
	if id, ok := t.(*ast.Ident); ok {
		recv = id.Name
	}
	if ptr {
		return "(*" + recv + ")." + d.Name.Name
	}
	return recv + "." + d.Name.Name
}

// calleeName is the bare function or method name a go statement starts,
// which the runtime matches against the first call on the new goroutine.
func calleeName(fun ast.Expr) string {
	switch x := fun.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.IndexExpr:
		return calleeName(x.X)
	case *ast.IndexListExpr:
		return calleeName(x.X)
	case *ast.ParenExpr:
		return calleeName(x.X)
	}
	return ""
}

// hasDirective reports a //go: directive (nosplit, linkname, ...). Adding a
// call to such a function can break the property the directive asserts.
func hasDirective(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		if strings.HasPrefix(c.Text, "//go:") {
			return true
		}
	}
	return false
}
