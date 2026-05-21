package predicate

// node is the predicate engine's typed IR. Unlike the raw gopher-lua
// AST, this IR carries only the constructs the engine accepts; the
// switch in eval.go has a default-panic case because every legal
// program produces only these node shapes.
//
// All node types are immutable after Compile.
type node interface {
	resultType() Type
	sealedNode()
}

// constNode is a baked-in scalar literal.
type constNode struct {
	v Value
}

func (n *constNode) resultType() Type { return n.v.Type() }
func (*constNode) sealedNode()        {}

// varNode reads a named binding at eval time.
type varNode struct {
	name string
	typ  Type
}

func (n *varNode) resultType() Type { return n.typ }
func (*varNode) sealedNode()        {}

// attrNode accesses a named field on a record-typed expression.
type attrNode struct {
	obj  node
	name string
	typ  Type
}

func (n *attrNode) resultType() Type { return n.typ }
func (*attrNode) sealedNode()        {}

// callNode invokes a host function. Args are positional, already
// type-checked at compile against the FuncSig.
type callNode struct {
	name string
	args []node
	typ  Type
}

func (n *callNode) resultType() Type { return n.typ }
func (*callNode) sealedNode()        {}

// tableArgNode is a named-args table literal used only inside a
// callNode's args. The keys are always strings; the values are
// always constants (enforced by the walker).
type tableArgNode struct {
	entries map[string]Value
}

func (n *tableArgNode) resultType() Type { return tableArgType{} }
func (*tableArgNode) sealedNode()        {}

// relationalNode is one of ==, ~=, <, <=, >, >=.
type relationalNode struct {
	op       string
	lhs, rhs node
}

func (*relationalNode) resultType() Type { return BoolType }
func (*relationalNode) sealedNode()      {}

// logicalNode is one of `and`, `or`. Lua-style short-circuit
// evaluation is preserved (and returns rhs only if lhs is truthy).
type logicalNode struct {
	op       string
	lhs, rhs node
}

func (*logicalNode) resultType() Type { return BoolType }
func (*logicalNode) sealedNode()      {}

// notNode is unary `not`.
type notNode struct {
	expr node
}

func (*notNode) resultType() Type { return BoolType }
func (*notNode) sealedNode()      {}

// tableArgType is an internal type carried only by table-literal
// nodes passed as function arguments. It is not in the public Type
// surface; callers can't observe or declare it.
type tableArgType struct{}

func (tableArgType) typeName() string       { return "table-arg" }
func (tableArgType) equalsType(o Type) bool { _, ok := o.(tableArgType); return ok }
func (tableArgType) sealedType()            {}
