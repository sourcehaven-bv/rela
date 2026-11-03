package predicate

import (
	"errors"
	"fmt"
)

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
// function returning the wrong type, a nil operand (see [ErrNilOperand]),
// or the per-Eval step budget being exhausted.
type EvalError struct {
	Reason string
	cause  error
}

func (e *EvalError) Error() string {
	return "predicate: eval error: " + e.Reason
}

// Unwrap returns the cause of the failure: [ErrNilOperand] or nil.
func (e *EvalError) Unwrap() error { return e.cause }

// ErrNilOperand is the cause of an [EvalError] for an operator that got
// nil where it needs a value: an ordered comparison, arithmetic,
// concatenation, a boolean operator, or a host function that fails with a
// nil argument. A nil usually stands for an absent
// optional value, so a caller that reads "absent" as "does not hold"
// tests for it with errors.Is and treats every other EvalError as a real
// failure.
var ErrNilOperand = errors.New("predicate: nil operand")

// nilOperand is the EvalError for a nil operand, with reason.
func nilOperand(reason string) *EvalError { return &EvalError{Reason: reason, cause: ErrNilOperand} }

// isNilValue reports whether v is the predicate nil.
func isNilValue(v Value) bool {
	_, ok := v.(Nil)
	return ok
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
