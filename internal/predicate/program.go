package predicate

// Program is a compiled predicate, ready for repeated evaluation.
//
// A Program is immutable after Compile. It carries no mutable state,
// no caches, and no per-instance memoization. Multiple goroutines
// may call Eval on the same Program concurrently with their own
// Bindings.
type Program struct {
	root      node
	resultTyp Type
	env       *Env
}

// ResultType returns the static type of the program's top-level
// expression.
func (p *Program) ResultType() Type { return p.resultTyp }

// EvalOption configures a single Eval call. Options stack
// left-to-right; later options override earlier ones.
type EvalOption func(*evalOptions)

type evalOptions struct {
	stepBudget int
}

// defaultStepBudget is the per-Eval node-visit cap. Tuned generous
// for hand-written rules; aggressive against adversarial input.
const defaultStepBudget = 10_000

// WithStepBudget overrides the per-Eval step budget. Must be > 0;
// values <= 0 are clamped to 1 so a misconfigured caller cannot
// disable the budget.
func WithStepBudget(n int) EvalOption {
	return func(o *evalOptions) {
		if n <= 0 {
			n = 1
		}
		o.stepBudget = n
	}
}
