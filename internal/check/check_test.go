package check

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixtures not available on Windows")
	}
}

// fakeChecker is a rela-format checker as an inline sh script, so it needs no
// file the sandbox would have to expose. It reports the input file's type
// directory and base name and the {type} argument in a finding on line 1.
func fakeChecker() metamodel.CheckerDef {
	script := `printf '{"findings":[{"rule":"Fake.Echo","severity":"warning","message":"%s/%s %s","line":1,"column":1,"length":3}]}' "$(basename "$(dirname "$1")")" "$(basename "$1")" "$2"`
	return metamodel.CheckerDef{
		Command: []string{"sh", "-c", script, "sh", cmdexec.TemplateIn, TemplateType},
		Format:  metamodel.CheckerFormatRela,
	}
}

func TestCheck_InputNameAndTypePlaceholder(t *testing.T) {
	skipOnWindows(t)
	res, err := NewEngine().Check(context.Background(),
		map[string]metamodel.CheckerDef{"fake": fakeChecker()}, "procedure", "PROC-1", []byte("Wij doen altijd alles."))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %+v", res.Findings)
	}
	f := res.Findings[0]
	if f.Message != "procedure/PROC-1.md procedure" || f.Checker != "fake" || f.Match != "Wij" {
		t.Errorf("finding = %+v", f)
	}
}

func TestCheck_EmptyBodyRunsNothing(t *testing.T) {
	fail := metamodel.CheckerDef{Command: []string{"definitely-not-a-real-binary-xyz"}, Format: metamodel.CheckerFormatRela}
	res, err := NewEngine().Check(context.Background(), map[string]metamodel.CheckerDef{"x": fail}, "task", "TASK-1", []byte(" \n\t"))
	if err != nil || len(res.Findings) != 0 || len(res.Errors) != 0 {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestCheck_UnsafeTypeRefused(t *testing.T) {
	_, err := NewEngine().Check(context.Background(),
		map[string]metamodel.CheckerDef{"fake": fakeChecker()}, "../etc", "X-1", []byte("x"))
	if err == nil {
		t.Error("unsafe type name accepted")
	}
}

func TestCheck_OneFailingCheckerDoesNotHideOthers(t *testing.T) {
	skipOnWindows(t)
	checkers := map[string]metamodel.CheckerDef{
		"a-broken":     {Command: []string{"sh", "-c", "echo boom >&2; exit 3"}, Format: metamodel.CheckerFormatRela},
		"b-garbage":    {Command: []string{"sh", "-c", "echo not-json"}, Format: metamodel.CheckerFormatRela},
		"c-unaccepted": {Command: []string{"sh", "-c", "echo '{}'; exit 1"}, Format: metamodel.CheckerFormatVale},
		"d-accepted":   {Command: []string{"sh", "-c", "echo '{}'; exit 1"}, Format: metamodel.CheckerFormatVale, ExitCodes: []int{0, 1}},
		"e-fake":       fakeChecker(),
	}
	res, err := NewEngine().Check(context.Background(), checkers, "task", "TASK-1", []byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Checker != "e-fake" {
		t.Errorf("findings = %+v", res.Findings)
	}
	var names []string
	for _, e := range res.Errors {
		names = append(names, e.Checker)
	}
	if strings.Join(names, ",") != "a-broken,b-garbage,c-unaccepted" {
		t.Errorf("errors = %v", res.Errors)
	}
	var exit *exec.ExitError
	if !errors.As(res.Errors[0], &exit) || exit.ExitCode() != 3 {
		t.Errorf("first error should carry the exit code: %v", res.Errors[0])
	}
}

func TestCheck_Timeout(t *testing.T) {
	skipOnWindows(t)
	def := metamodel.CheckerDef{Command: []string{"sleep", "10"}, Format: metamodel.CheckerFormatRela, Timeout: "100ms"}
	res, err := NewEngine().Check(context.Background(), map[string]metamodel.CheckerDef{"slow": def}, "task", "TASK-1", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Error(), "timed out after 100ms") {
		t.Errorf("errors = %v", res.Errors)
	}
}

func TestCheck_CancelledContextIsNotACheckerError(t *testing.T) {
	skipOnWindows(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewEngine().Check(ctx, map[string]metamodel.CheckerDef{"fake": fakeChecker()}, "task", "TASK-1", []byte("x"))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestCheck_UnsafeIDFallsBack(t *testing.T) {
	skipOnWindows(t)
	for _, id := range []string{"", "../x", "a b", ".hidden"} {
		res, err := NewEngine().Check(context.Background(),
			map[string]metamodel.CheckerDef{"fake": fakeChecker()}, "task", id, []byte("abc"))
		if err != nil || len(res.Findings) != 1 || res.Findings[0].Message != "task/body.md task" {
			t.Errorf("id %q: res = %+v, err = %v", id, res, err)
		}
	}
}

func TestAnalyzeCheckers(t *testing.T) {
	m := &metamodel.Metamodel{Checkers: map[string]metamodel.CheckerDef{
		"on":  {Analyze: true},
		"off": {},
	}}
	got := AnalyzeCheckers(m)
	if _, ok := got["on"]; !ok || len(got) != 1 {
		t.Errorf("got %v", got)
	}
	if AnalyzeCheckers(nil) != nil {
		t.Error("nil metamodel")
	}
}

// TestCheck_ValeTypeSections runs the real Vale binary through the real
// runner: a `[**/<type>/*.md]` section must fire on the path cmdexec passes,
// and not for other types. Vale matches against the absolute path, which is
// why docs/checkers.md prescribes the `**/` form.
func TestCheck_ValeTypeSections(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("vale"); err != nil {
		t.Skip("vale not available")
	}
	dir := t.TempDir()
	styles := filepath.Join(dir, "styles", "T")
	if err := os.MkdirAll(styles, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(styles, "Base.yml"), "extends: existence\nmessage: \"base %s\"\nlevel: warning\ntokens: [altijd]\n")
	write(filepath.Join(styles, "Typ.yml"), "extends: existence\nmessage: \"type %s\"\nlevel: error\ntokens: [nooit]\n")
	cfg := filepath.Join(dir, ".vale.ini")
	write(cfg, "StylesPath = "+filepath.Join(dir, "styles")+"\n[*.md]\nBasedOnStyles = T\nT.Typ = NO\n"+
		"[**/procedure/*.md]\nBasedOnStyles = T\nT.Typ = YES\n")

	if runtime.GOOS == "linux" {
		cmdexec.SetHostReadOnly(cmdexec.PurposeCheck, []string{dir})
		t.Cleanup(func() { cmdexec.SetHostReadOnly(cmdexec.PurposeCheck, nil) })
	}
	engine := NewEngine()
	checkers := map[string]metamodel.CheckerDef{"vale": {
		Command: []string{"vale", "--output=JSON", "--no-exit", "--config=" + cfg, cmdexec.TemplateIn},
		Format:  metamodel.CheckerFormatVale,
	}}
	body := []byte("Wij doen altijd en nooit iets.\n")

	rules := func(typeName string) string {
		t.Helper()
		res, err := engine.Check(context.Background(), checkers, typeName, "X-1", body)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %v", typeName, res.Errors)
		}
		var out []string
		for _, f := range res.Findings {
			out = append(out, f.Rule+"="+f.Match)
		}
		return strings.Join(out, ",")
	}
	if got := rules("procedure"); got != "T.Base=altijd,T.Typ=nooit" {
		t.Errorf("procedure: %s", got)
	}
	if got := rules("task"); got != "T.Base=altijd" {
		t.Errorf("task: %s", got)
	}
}
