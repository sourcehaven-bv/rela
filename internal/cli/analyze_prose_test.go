package cli

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// fakeProseChecker reports every line containing "altijd" in the {in} file
// as a rela-format finding. It uses only sh builtins: in the Linux sandbox
// /etc is absent, so a tool reached through /etc/alternatives (awk on
// Debian/Ubuntu) does not resolve.
var fakeProseChecker = metamodel.CheckerDef{
	Command: []string{"sh", "-c", `
printf '{"findings":['; n=0; sep=''
while IFS= read -r line; do
  n=$((n+1))
  case "$line" in *altijd*)
    pre=${line%%altijd*}
    printf '%s{"rule":"Fake.Absolute","severity":"warning","message":"absolute claim","line":%d,"column":%d,"length":6,"replacements":["meestal"]}' "$sep" "$n" "$((${#pre}+1))"
    sep=',';;
  esac
done < "$1"
printf ']}'`, "sh", cmdexec.TemplateIn},
	Format:  metamodel.CheckerFormatRela,
	Analyze: true,
}

func proseCLIBundles(t *testing.T, checkers map[string]metamodel.CheckerDef) *cliBundles {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixtures not available on Windows")
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"procedure": {
				Label: "Procedure", IDPrefix: "PROC-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
			"task": {
				Label: "Task", IDPrefix: "TASK-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			},
		},
		Checkers: checkers,
	}
	ss := newStoreSeeder(meta)
	for _, e := range []*entity.Entity{
		{ID: "PROC-1", Type: "procedure", Properties: map[string]any{"title": "Back-ups"}, Content: "We controleren altijd de back-up.\n"},
		{ID: "PROC-2", Type: "procedure", Properties: map[string]any{"title": "Clean"}, Content: "Elk kwartaal.\n"},
		{ID: "TASK-1", Type: "task", Properties: map[string]any{"title": "Run"}, Content: "Eerst dit.\nDan altijd dat.\n"},
	} {
		if err := ss.s.CreateEntity(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	return ss.build(t)
}

func TestAnalyzeProse_Text(t *testing.T) {
	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"fake": fakeProseChecker})
	buf := withOutput(t, output.FormatTable)
	if err := (&AnalyzeProseCmd{}).Run(context.Background(), b.read); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	for _, w := range []string{
		"PROC-1 (procedure)",
		"1:16    warning    Fake.Absolute  absolute claim (use: meestal)",
		"TASK-1 (task)",
		"2:5 ",
		"Found 2 prose finding(s) in 2 entities",
	} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, buf)
		}
	}
	if strings.Contains(buf.String(), "PROC-2") {
		t.Errorf("entity without findings listed:\n%s", buf)
	}
}

func TestAnalyzeProse_TypeFilterAndJSON(t *testing.T) {
	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"fake": fakeProseChecker})
	buf := withOutput(t, output.FormatJSON)
	if err := (&AnalyzeProseCmd{Type: "task"}).Run(context.Background(), b.read); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	var res struct {
		Status  string `json:"status"`
		Count   int    `json:"count"`
		Details []struct {
			ID       string `json:"id"`
			Findings []struct {
				Match string `json:"match"`
				Line  int    `json:"line"`
			} `json:"findings"`
		} `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	if res.Status != "warning" || res.Count != 1 || len(res.Details) != 1 || res.Details[0].ID != "TASK-1" ||
		res.Details[0].Findings[0].Match != "altijd" || res.Details[0].Findings[0].Line != 2 {

		t.Errorf("res = %+v", res)
	}
}

func TestAnalyzeProse_UnknownType(t *testing.T) {
	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"fake": fakeProseChecker})
	withOutput(t, output.FormatTable)
	if err := (&AnalyzeProseCmd{Type: "nope"}).Run(context.Background(), b.read); err == nil {
		t.Error("unknown type accepted")
	}
}

func TestAnalyzeProse_OnlyAnalyzeCheckersRun(t *testing.T) {
	off := fakeProseChecker
	off.Analyze = false
	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"fake": off})
	buf := withOutput(t, output.FormatTable)
	if err := (&AnalyzeProseCmd{}).Run(context.Background(), b.read); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	if !strings.Contains(buf.String(), "No checkers with analyze: true configured") {
		t.Errorf("output:\n%s", buf)
	}
}

func TestAnalyzeProse_CheckerFailureIsAnError(t *testing.T) {
	broken := metamodel.CheckerDef{Command: []string{"sh", "-c", "exit 4"}, Format: metamodel.CheckerFormatRela, Analyze: true}
	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"broken": broken})
	buf := withOutput(t, output.FormatTable)
	err := (&AnalyzeProseCmd{}).Run(context.Background(), b.read)
	if err == nil || !strings.Contains(err.Error(), "3 checker run(s) failed") {
		t.Errorf("err = %v", err)
	}
	if !strings.Contains(buf.String(), `checker "broken"`) {
		t.Errorf("output lacks the checker error:\n%s", buf)
	}
}

func TestAnalyzeAll_ProseOnlyWithFlag(t *testing.T) {
	broken := metamodel.CheckerDef{Command: []string{"sh", "-c", "exit 4"}, Format: metamodel.CheckerFormatRela, Analyze: true}
	ctx := context.Background()

	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"broken": broken})
	buf := withOutput(t, output.FormatJSON)
	if err := (&AnalyzeAllCmd{}).Run(ctx, b.read, b.analysis); err != nil {
		t.Fatalf("analyze all without --prose ran a checker: %v", err)
	}
	if strings.Contains(buf.String(), `"prose"`) {
		t.Errorf("prose count without --prose:\n%s", buf)
	}

	b = proseCLIBundles(t, map[string]metamodel.CheckerDef{"fake": fakeProseChecker})
	buf = withOutput(t, output.FormatJSON)
	if err := (&AnalyzeAllCmd{Prose: true}).Run(ctx, b.read, b.analysis); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	var res struct {
		Details struct {
			Prose *int `json:"prose"`
		} `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	if res.Details.Prose == nil || *res.Details.Prose != 2 {
		t.Errorf("prose = %v\n%s", res.Details.Prose, buf)
	}

	buf = withOutput(t, output.FormatTable)
	if err := (&AnalyzeAllCmd{Prose: true}).Run(ctx, b.read, b.analysis); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	for _, w := range []string{"Prose: 2", "Prose Checks", "PROC-1 (procedure)"} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, buf)
		}
	}
}

func TestAnalyzeProse_MissingBinaryFailsOnce(t *testing.T) {
	missing := metamodel.CheckerDef{
		Command: []string{"rela-no-such-checker", cmdexec.TemplateIn}, Format: metamodel.CheckerFormatRela, Analyze: true,
	}
	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"missing": missing})
	buf := withOutput(t, output.FormatTable)
	err := (&AnalyzeProseCmd{}).Run(context.Background(), b.read)
	if err == nil || !strings.Contains(err.Error(), `checker "missing"`) || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
	if strings.Contains(buf.String(), "PROC-1") {
		t.Errorf("checked entities despite the missing binary:\n%s", buf)
	}
}

func TestAnalyzeProse_JSONFailureStatusAndAllDetails(t *testing.T) {
	broken := metamodel.CheckerDef{Command: []string{"sh", "-c", "exit 4"}, Format: metamodel.CheckerFormatRela, Analyze: true}
	ctx := context.Background()

	b := proseCLIBundles(t, map[string]metamodel.CheckerDef{"broken": broken})
	buf := withOutput(t, output.FormatJSON)
	if err := (&AnalyzeProseCmd{Type: "task"}).Run(ctx, b.read); err == nil {
		t.Fatal("checker failure not an error")
	}
	var res struct {
		Status   string `json:"status"`
		Coverage string `json:"coverage"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	if res.Status != "error" || res.Coverage == "" {
		t.Errorf("res = %+v", res)
	}

	b = proseCLIBundles(t, map[string]metamodel.CheckerDef{"broken": broken})
	buf = withOutput(t, output.FormatJSON)
	if err := (&AnalyzeAllCmd{Prose: true}).Run(ctx, b.read, b.analysis); err == nil {
		t.Fatal("analyze all --prose: checker failure not an error")
	}
	var all struct {
		Status  string `json:"status"`
		Details struct {
			ProseEntities []struct {
				ID     string   `json:"id"`
				Errors []string `json:"errors"`
			} `json:"prose_entities"`
		} `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &all); err != nil {
		t.Fatalf("%v\n%s", err, buf)
	}
	if all.Status != "error" || len(all.Details.ProseEntities) != 3 ||
		!strings.Contains(all.Details.ProseEntities[0].Errors[0], `checker "broken"`) {

		t.Errorf("all = %+v", all)
	}
}
