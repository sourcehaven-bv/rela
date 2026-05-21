---
id: RR-PCLY
type: review-response
title: Func should be an interface; Eval should take context.Context (now-or-never)
finding: 'bindings.go: type Func is `func(args []Value) (Value, error)`. Two costs: (1) host implementations can''t carry metadata, audit hooks, or context; (2) no place for cancellation. ACL host functions (has_role, has_relation) will traverse the store, which is context-shaped — they''d close over a stored ctx, the well-known antipattern. Fix: `type Func interface { Call(ctx context.Context, args []Value) (Value, error) }` plus a FuncOf adapter for callers who prefer closures. Eval becomes `(b Bindings, ctx context.Context, opts ...) (Value, error)`. This is a now-or-never change — after ACL wires up, every host-function impl is a breaking API change.'
severity: significant
resolution: 'Reshaped Func as an interface: `type Func interface { Call(ctx context.Context, args []Value) (Value, error) }` with a `FuncFunc` adapter for closure callers. Eval signature is now `Eval(ctx context.Context, b *Bindings, opts ...EvalOption)`; ctx is threaded through every eval method (not stored on the struct — golangci-lint containedctx). New TestEval_ThreadsContext verifies the caller-supplied ctx reaches host-fn implementations.'
status: addressed
---
