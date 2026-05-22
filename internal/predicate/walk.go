package predicate

import (
	"fmt"

	"github.com/yuin/gopher-lua/ast"
)

func (w *walker) walkIdent(e *ast.IdentExpr) (node, error) {
	t, ok := w.env.lookupVar(e.Value)
	if !ok {
		// IdentExpr is also how a function reference appears when the
		// caller writes the name without parens. We don't accept bare
		// function references — every reachable identifier must be a
		// declared variable. (A function call goes through walkCall.)
		if _, isFunc := w.env.lookupFunc(e.Value); isFunc {
			return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("function %q must be called, not referenced as a value", e.Value)}
		}
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("unknown identifier %q", e.Value)}
	}
	return &varNode{name: e.Value, typ: t}, nil
}

func (w *walker) walkAttrGet(e *ast.AttrGetExpr) (node, error) {
	// Per-field invariant (RR-V0OE): the Key must be a StringExpr. Both
	// `entity.status` (dot-sugar) and `entity['status']` (bracket form
	// with a string literal key) lower to the same AttrGetExpr shape in
	// gopher-lua, so we accept both; they are semantically identical.
	// A non-literal key, e.g. `entity[has_role('x')]`, lowers to
	// AttrGetExpr{Key: *FuncCallExpr} and is rejected here.
	keyStr, ok := e.Key.(*ast.StringExpr)
	if !ok {
		return nil, &CompileError{Line: e.Line(), Reason: "computed attribute access (entity[expr]) is not allowed; use a string literal key"}
	}

	obj, err := w.walkExpr(e.Object)
	if err != nil {
		return nil, err
	}
	rt, ok := obj.resultType().(RecordType)
	if !ok {
		return nil, &CompileError{Line: e.Line(), Reason: "attribute access requires a record value, got " + obj.resultType().typeName()}
	}
	ft, ok := rt[keyStr.Value]
	if !ok {
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("unknown attribute %q on record", keyStr.Value)}
	}
	return &attrNode{obj: obj, name: keyStr.Value, typ: ft}, nil
}

func (w *walker) walkRelational(e *ast.RelationalOpExpr) (node, error) {
	switch e.Operator {
	case "==", "~=", "<", "<=", ">", ">=":
		// allowed
	default:
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("unsupported relational operator %q", e.Operator)}
	}
	lhs, err := w.walkExpr(e.Lhs)
	if err != nil {
		return nil, err
	}
	rhs, err := w.walkExpr(e.Rhs)
	if err != nil {
		return nil, err
	}
	if err := checkRelational(e.Operator, lhs.resultType(), rhs.resultType(), e.Line()); err != nil {
		return nil, err
	}
	return &relationalNode{op: e.Operator, lhs: lhs, rhs: rhs}, nil
}

// checkRelational enforces the equality / ordering rules documented in
// doc.go. == and ~= allow nil-vs-anything (always false unless both
// are nil); other pairings must be same-type. Ordered comparisons
// require both number or both string.
func checkRelational(op string, lt, rt Type, line int) error {
	switch op {
	case "==", "~=":
		// Either operand may be nil; that's the only legal cross-type
		// comparison. Otherwise both sides must be the same scalar
		// type. Records and lists can't be compared.
		if _, ok := lt.(RecordType); ok {
			return &CompileError{Line: line, Reason: "records cannot be compared with == or ~="}
		}
		if _, ok := rt.(RecordType); ok {
			return &CompileError{Line: line, Reason: "records cannot be compared with == or ~="}
		}
		if _, ok := lt.(ListType); ok {
			return &CompileError{Line: line, Reason: "lists cannot be compared with == or ~="}
		}
		if _, ok := rt.(ListType); ok {
			return &CompileError{Line: line, Reason: "lists cannot be compared with == or ~="}
		}
		// Nil is the only legal cross-type. Same-type for the rest.
		if isNil(lt) || isNil(rt) {
			return nil
		}
		if !lt.equalsType(rt) {
			return &CompileError{Line: line, Reason: fmt.Sprintf("cannot compare %s with %s", lt.typeName(), rt.typeName())}
		}
		return nil
	case "<", "<=", ">", ">=":
		if !lt.equalsType(rt) {
			return &CompileError{Line: line, Reason: fmt.Sprintf("ordered comparison requires same type, got %s and %s", lt.typeName(), rt.typeName())}
		}
		if !lt.equalsType(NumberType) && !lt.equalsType(StringType) {
			return &CompileError{Line: line, Reason: "ordered comparison requires number or string, got " + lt.typeName()}
		}
		return nil
	}
	return &CompileError{Line: line, Reason: fmt.Sprintf("unknown relational operator %q", op)}
}

func isNil(t Type) bool { return t.equalsType(NilType) }

func (w *walker) walkLogical(e *ast.LogicalOpExpr) (node, error) {
	switch e.Operator {
	case "and", "or":
		// allowed
	default:
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("unsupported logical operator %q", e.Operator)}
	}
	lhs, err := w.walkExpr(e.Lhs)
	if err != nil {
		return nil, err
	}
	rhs, err := w.walkExpr(e.Rhs)
	if err != nil {
		return nil, err
	}
	if !lhs.resultType().equalsType(BoolType) {
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("'%s' requires bool on left, got %s", e.Operator, lhs.resultType().typeName())}
	}
	if !rhs.resultType().equalsType(BoolType) {
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("'%s' requires bool on right, got %s", e.Operator, rhs.resultType().typeName())}
	}
	return &logicalNode{op: e.Operator, lhs: lhs, rhs: rhs}, nil
}

func (w *walker) walkNot(e *ast.UnaryNotOpExpr) (node, error) {
	inner, err := w.walkExpr(e.Expr)
	if err != nil {
		return nil, err
	}
	if !inner.resultType().equalsType(BoolType) {
		return nil, &CompileError{Line: e.Line(), Reason: "'not' requires bool, got " + inner.resultType().typeName()}
	}
	return &notNode{expr: inner}, nil
}

func (w *walker) walkCall(e *ast.FuncCallExpr) (node, error) {
	// Method-call form t:m() is explicitly rejected; Receiver != nil
	// indicates the parser saw a colon.
	if e.Receiver != nil {
		return nil, &CompileError{Line: e.Line(), Reason: "method-call syntax (obj:method()) is not allowed"}
	}
	// AdjustRet flags multi-value return semantics (e.g. `f(g())`
	// where g returns multiple values that adjust to f's arity).
	// We only support single-value returns.
	if e.AdjustRet {
		return nil, &CompileError{Line: e.Line(), Reason: "multi-value return adjustment is not allowed"}
	}

	// The callee must be a bare identifier referring to a declared
	// function. We do not allow first-class function values: no
	// (expr)(args), no entity.method(args).
	ident, ok := e.Func.(*ast.IdentExpr)
	if !ok {
		return nil, &CompileError{Line: e.Line(), Reason: "function call must target a declared function name"}
	}
	sig, ok := w.env.lookupFunc(ident.Value)
	if !ok {
		// Variables-named-as-funcs go via the "unknown function"
		// branch so the message points the author at the right fix.
		if _, isVar := w.env.lookupVar(ident.Value); isVar {
			return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("%q is a variable, not a function", ident.Value)}
		}
		return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("unknown function %q", ident.Value)}
	}

	args, err := w.walkCallArgs(e, sig)
	if err != nil {
		return nil, err
	}
	return &callNode{name: ident.Value, args: args, typ: sig.Return}, nil
}

func (w *walker) walkCallArgs(e *ast.FuncCallExpr, sig FuncSig) ([]node, error) {
	// Special case: a single table-literal argument is the named-args
	// form has_relation('x', {status='open'}). gopher-lua produces a
	// FuncCallExpr with Args = []Expr{*TableExpr{...}} for that.
	// We translate the table into a tableArgNode and type-check it
	// against the corresponding FuncSig parameter (also a table-arg).
	out := make([]node, len(e.Args))
	for i, arg := range e.Args {
		if tbl, ok := arg.(*ast.TableExpr); ok {
			n, err := w.walkTableArg(tbl)
			if err != nil {
				return nil, err
			}
			out[i] = n
		} else {
			n, err := w.walkExpr(arg)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
	}

	// Arity check.
	fixed := len(sig.Params)
	if sig.Variadic == nil {
		if len(out) != fixed {
			return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("function %q: expected %d arg(s), got %d", funcNameOf(e), fixed, len(out))}
		}
	} else {
		if len(out) < fixed {
			return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("function %q: expected at least %d arg(s), got %d", funcNameOf(e), fixed, len(out))}
		}
	}

	// Type check fixed params.
	for i := range fixed {
		expected := sig.Params[i]
		got := out[i].resultType()
		if !typeAccepts(expected, got) {
			return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("function %q: param %d: expected %s, got %s", funcNameOf(e), i, expected.typeName(), got.typeName())}
		}
	}
	// Type check variadic params.
	if sig.Variadic != nil {
		for i := fixed; i < len(out); i++ {
			got := out[i].resultType()
			if !typeAccepts(sig.Variadic, got) {
				return nil, &CompileError{Line: e.Line(), Reason: fmt.Sprintf("function %q: variadic arg %d: expected %s, got %s", funcNameOf(e), i, sig.Variadic.typeName(), got.typeName())}
			}
		}
	}
	return out, nil
}

func funcNameOf(e *ast.FuncCallExpr) string {
	if id, ok := e.Func.(*ast.IdentExpr); ok {
		return id.Value
	}
	return "<unknown>"
}

// typeAccepts reports whether a value of type `got` can flow into a
// position expecting type `expected`.
//
//   - AnyType accepts any value.
//   - A RecordType param accepts a tableArgType (the {key='value'} named
//     args form). Per-field validation is the host function's job; we
//     don't reach into the table at compile time because the named-args
//     form is intentionally untyped on the call side.
//   - Otherwise the types must match by equalsType.
func typeAccepts(expected, got Type) bool {
	if expected.equalsType(AnyType) {
		return true
	}
	if _, isRecord := expected.(RecordType); isRecord {
		// A RecordType param accepts any Record-shaped value: a
		// table-arg literal (named-args form) or another record
		// from a binding. Field-level structural checks are not
		// done at this layer; declaring RecordType{} means "any
		// record." A future change could narrow this to require
		// the actual record's fields to be a superset of the
		// declared shape, but the current ACL use cases don't
		// need that yet.
		if _, isTableArg := got.(tableArgType); isTableArg {
			return true
		}
		if _, isRecordGot := got.(RecordType); isRecordGot {
			return true
		}
	}
	return expected.equalsType(got)
}

// walkTableArg accepts a TableExpr only in the form {key='value', ...}
// — string keys via the sugar form, constant values.
func (w *walker) walkTableArg(t *ast.TableExpr) (node, error) {
	if err := w.enter(t.Line()); err != nil {
		return nil, err
	}
	defer w.leave()

	entries := make(map[string]Value, len(t.Fields))
	for _, f := range t.Fields {
		if f.Key == nil {
			return nil, &CompileError{Line: t.Line(), Reason: "positional table fields are not allowed; use {key='value'}"}
		}
		keyStr, ok := f.Key.(*ast.StringExpr)
		if !ok {
			return nil, &CompileError{Line: t.Line(), Reason: "table keys must be bare identifiers ({key='value'} form)"}
		}
		v, err := constValueOf(f.Value)
		if err != nil {
			return nil, &CompileError{Line: t.Line(), Reason: fmt.Sprintf("table value for %q: %s", keyStr.Value, err.Error())}
		}
		if _, dup := entries[keyStr.Value]; dup {
			return nil, &CompileError{Line: t.Line(), Reason: fmt.Sprintf("duplicate table key %q", keyStr.Value)}
		}
		entries[keyStr.Value] = v
	}
	return &tableArgNode{entries: entries}, nil
}

// constValueOf accepts only ConstExpr-style AST nodes for table values.
// Nested expressions are explicitly rejected to keep the named-args
// form simple and the IR validation trivial.
func constValueOf(e ast.Expr) (Value, error) {
	switch x := e.(type) {
	case *ast.TrueExpr:
		return NewBool(true), nil
	case *ast.FalseExpr:
		return NewBool(false), nil
	case *ast.NilExpr:
		return NewNil(), nil
	case *ast.NumberExpr:
		f, err := parseLuaNumber(x.Value)
		if err != nil {
			return nil, err
		}
		return NewNumber(f), nil
	case *ast.StringExpr:
		return NewString(x.Value), nil
	default:
		return nil, fmt.Errorf("must be a constant (string, number, bool, or nil); got %s", astKindName(e))
	}
}

// astKindName maps gopher-lua AST node types to user-friendly strings
// for error messages. Falls back to a generic label for nodes we
// don't have a specific name for — but every kind we'd encounter in
// the predicate-language grammar surface should have a clean name.
func astKindName(e ast.Expr) string {
	switch e.(type) {
	case *ast.FuncCallExpr:
		return "function call"
	case *ast.IdentExpr:
		return "identifier"
	case *ast.AttrGetExpr:
		return "attribute access"
	case *ast.TableExpr:
		return "table literal"
	case *ast.RelationalOpExpr:
		return "comparison expression"
	case *ast.LogicalOpExpr:
		return "boolean expression"
	case *ast.UnaryNotOpExpr:
		return "not-expression"
	case *ast.ArithmeticOpExpr:
		return "arithmetic expression"
	case *ast.StringConcatOpExpr:
		return "string concatenation"
	case *ast.UnaryMinusOpExpr:
		return "unary minus"
	case *ast.UnaryLenOpExpr:
		return "length operator"
	case *ast.FunctionExpr:
		return "function literal"
	case *ast.Comma3Expr:
		return "varargs (...)"
	}
	return "non-constant expression"
}
