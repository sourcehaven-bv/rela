package predicate

import "fmt"

// ParseError reports a failure inside gopher-lua's parser. Line and
// Col are best-effort: they reflect the position the parser reported,
// adjusted for the synthetic "return " prefix we prepend.
type ParseError struct {
	Line int
	Col  int
	Msg  string
}

func (e *ParseError) Error() string {
	pos := formatPos(e.Line, e.Col)
	if pos == "" {
		return "predicate: parse error: " + e.Msg
	}
	return "predicate: parse error " + pos + ": " + e.Msg
}

// CompileError reports a failure to translate the parsed AST into the
// predicate IR: an unsupported AST node, a per-field invariant
// violation, an unknown symbol, a type mismatch, or a budget overrun.
type CompileError struct {
	Line   int
	Col    int
	Reason string
}

func (e *CompileError) Error() string {
	pos := formatPos(e.Line, e.Col)
	if pos == "" {
		return "predicate: compile error: " + e.Reason
	}
	return "predicate: compile error " + pos + ": " + e.Reason
}

// EvalError reports a failure during Eval — a missing binding, a host
// function returning the wrong type, or the per-Eval step budget
// being exhausted.
type EvalError struct {
	Reason string
}

func (e *EvalError) Error() string {
	return "predicate: eval error: " + e.Reason
}

// formatPos renders a line/col pair the way humans read it. Col 0 is
// the "unknown column" sentinel that gopher-lua's AST returns for
// every node — we omit it rather than print a misleading "col 0".
func formatPos(line, col int) string {
	if line <= 0 {
		return ""
	}
	if col <= 0 {
		return fmt.Sprintf("at line %d", line)
	}
	return fmt.Sprintf("at line %d, col %d", line, col)
}
