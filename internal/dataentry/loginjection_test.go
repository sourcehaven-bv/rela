package dataentry

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSlogMessagesAreConstant is the structural invariant behind excluding
// gosec's G706 (log injection) for this package.
//
// G706 flags every request-derived value that reaches slog.Info/Warn/Error/
// Debug, because its taint analysis stops at the slog call and cannot see the
// handler that encodes the record. Log injection needs an attacker-controlled
// newline to land in the output *unescaped*. Two properties prevent that here:
//
//  1. Every slog call in this package passes a CONSTANT string literal as the
//     message; user data only ever travels as a structured key/value attribute.
//     This test enforces that property.
//  2. Attribute values are escaped by the handler. Every entry point installs
//     slog.NewTextHandler (internal/cli/kong.go, cmd/rela-server, cmd/rela-desktop),
//     which quotes any value containing spaces, quotes or control characters —
//     the same guarantee gosec already accepts from strconv.Quote, which is what
//     TextHandler uses internally. TestSlogTextHandlerEscapesNewlines pins it.
//
// If this test fails, do NOT re-exclude the finding: a non-constant slog message
// is a genuine log-injection sink. Move the dynamic part into an attribute.
func TestSlogMessagesAreConstant(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(root, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "slog" {
				return true
			}
			switch sel.Sel.Name {
			case "Info", "Warn", "Error", "Debug",
				"InfoContext", "WarnContext", "ErrorContext", "DebugContext":
			default:
				return true
			}
			// Locate the message argument: it follows ctx for the *Context forms.
			msgIdx := 0
			if strings.HasSuffix(sel.Sel.Name, "Context") {
				msgIdx = 1
			}
			if len(call.Args) <= msgIdx {
				return true
			}
			// A constant expression is safe: it is fixed at compile time and
			// cannot carry request data. That includes a plain literal and a
			// concatenation of literals ("foo " + "bar"), which is how longer
			// operator-facing messages are wrapped in this package.
			if isConstantString(call.Args[msgIdx]) {
				return true
			}
			msg := call.Args[msgIdx]
			t.Errorf("%s: slog.%s message is not a constant string literal — "+
				"user data in a log MESSAGE is a real log-injection sink (G706). "+
				"Pass it as a structured attribute instead.",
				fset.Position(msg.Pos()), sel.Sel.Name)
			return true
		})
	}
}

// isConstantString reports whether e is a compile-time string constant: a
// string literal, or a binary concatenation of string constants. Anything
// else (a variable, a function call such as fmt.Sprintf, a field selector)
// could carry request-derived data into the log message itself.
func isConstantString(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.BasicLit:
		return v.Kind == token.STRING
	case *ast.ParenExpr:
		return isConstantString(v.X)
	case *ast.BinaryExpr:
		return v.Op == token.ADD && isConstantString(v.X) && isConstantString(v.Y)
	default:
		return false
	}
}

// TestSlogTextHandlerEscapesNewlines pins the encoding guarantee that makes
// request-derived data safe as an slog ATTRIBUTE value: TextHandler quotes and
// escapes control characters, so an injected newline cannot start a forged log
// record. This is the property gosec's G706 taint analysis cannot observe.
func TestSlogTextHandlerEscapesNewlines(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	forged := "bob\nlevel=ERROR msg=\"forged entry\" user=admin"
	logger.Warn("acl: probe", "user", forged, "path", "/x\ny=z")

	out := buf.String()
	if strings.Contains(out, "\n") && strings.Count(out, "\n") != 1 {
		t.Errorf("expected a single trailing newline, got %d in %q", strings.Count(out, "\n"), out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected trailing newline, got %q", out)
	}
	if strings.Contains(out, `\n`) == false {
		t.Errorf("expected the injected newline to be escaped as \\n, got %q", out)
	}
	// The forged record must not appear as a standalone entry.
	body := strings.TrimSuffix(out, "\n")
	if strings.Contains(body, "\n") {
		t.Errorf("injected newline broke the record onto a second line: %q", out)
	}
}
