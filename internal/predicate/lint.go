package predicate

// NamedSource pairs a predicate source string with a stable name the
// caller can use to identify the source in lint output.
type NamedSource struct {
	Name   string
	Source string
}

// Issue is a single compile failure: the source it came from and the
// parse / compile error that produced it. Err is one of *ParseError
// or *CompileError; use errors.As to inspect (RR-8VKE).
type Issue struct {
	Name string
	Err  error
}

// CompileAll compiles every source in turn against env and returns
// (a) the compiled programs in input order, with a nil entry where
// compile failed, and (b) one Issue per failed source.
//
// Intended for batch checking at policy-load time so a caller (e.g.
// an ACL loader in a future PR) can fail-fast on bad rules while
// keeping the successfully compiled programs ready for use — no
// double-parse needed (RR-LQE9).
func CompileAll(env *Env, sources []NamedSource, opts ...CompileOption) ([]*Program, []Issue) {
	progs := make([]*Program, len(sources))
	var issues []Issue
	for i, s := range sources {
		prog, err := Compile(env, s.Source, opts...)
		if err != nil {
			issues = append(issues, Issue{Name: s.Name, Err: err})
			continue
		}
		progs[i] = prog
	}
	return progs, issues
}
