package cli

import (
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Sourcehaven-BV/rela/internal/classification"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
)

const classificationTestSchema = `version: "1"
entities:
  person:
    label: Person
    id_prefix: P-
    properties:
      name: {type: string}
      email: {type: string}
  topic:
    label: Topic
    id_type: manual
    properties:
      title: {type: string}
relations:
  follows:
    label: follows
    from: [person]
    to: [topic]
    content: true
    properties:
      since: {type: string}
`

func classificationProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "schema.yaml"), []byte(classificationTestSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := projectPath
	projectPath = dir
	t.Cleanup(func() { projectPath = prev })
	return dir
}

func exitCode(err error) int {
	var exit *relaerrors.ExitError
	if stderrors.As(err, &exit) {
		return exit.Code
	}
	if err != nil {
		return -1
	}
	return 0
}

func TestClassificationCmd_SyncThenLint(t *testing.T) {
	dir := classificationProjectDir(t)
	withOutput(t, output.FormatTable)
	path := filepath.Join(dir, classification.FileName)

	stdout, err := captureStdout(t, (&ClassificationLintCmd{}).Run)
	if err != nil || !strings.Contains(stdout, "no classification.yaml") {
		t.Fatalf("lint without file: err=%v out=%q", err, stdout)
	}

	stdout, err = captureStdout(t, (&ClassificationSyncCmd{DryRun: true}).Run)
	if err != nil || !strings.Contains(stdout, "would change") {
		t.Fatalf("dry run: err=%v out=%q", err, stdout)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("dry run wrote the file")
	}

	stdout, err = captureStdout(t, (&ClassificationSyncCmd{}).Run)
	if err != nil || !strings.Contains(stdout, "created classification.yaml") {
		t.Fatalf("sync: err=%v out=%q", err, stdout)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"person:\n    name: needs-review\n    email: needs-review\n    body: needs-review",
		"follows:\n    since: needs-review\n    body: needs-review",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("file lacks %q:\n%s", want, data)
		}
	}

	stdout, err = captureStdout(t, (&ClassificationLintCmd{}).Run)
	if exitCode(err) != 1 || !strings.Contains(stdout, "assign.person.name") {
		t.Fatalf("lint after sync: exit=%d out=%q", exitCode(err), stdout)
	}

	reviewed := strings.ReplaceAll(string(data), "needs-review", "none")
	if writeErr := os.WriteFile(path, []byte(reviewed), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	stdout, err = captureStdout(t, (&ClassificationLintCmd{}).Run)
	if err != nil || !strings.Contains(stdout, "matches the schema") {
		t.Fatalf("lint after review: err=%v out=%q", err, stdout)
	}
	stdout, err = captureStdout(t, (&ClassificationSyncCmd{}).Run)
	if err != nil || !strings.Contains(stdout, "up to date") {
		t.Fatalf("second sync: err=%v out=%q", err, stdout)
	}
}

func TestClassificationCmd_JSON(t *testing.T) {
	dir := classificationProjectDir(t)
	buf := withOutput(t, output.FormatJSON)
	if err := (&ClassificationSyncCmd{}).Run(); err != nil {
		t.Fatal(err)
	}
	var res struct {
		Status  string         `json:"status"`
		Details syncResultJSON `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("sync json: %v\n%s", err, buf)
	}
	if !res.Details.Written || len(res.Details.Added) != 7 || res.Details.Stale == nil {
		t.Errorf("sync details = %+v", res.Details)
	}

	buf.Reset()
	data := "labels: {}\nassign:\n  gone:\n    x: none\n"
	if err := os.WriteFile(filepath.Join(dir, classification.FileName), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	err := (&ClassificationLintCmd{}).Run()
	if exitCode(err) != 1 {
		t.Fatalf("lint exit = %d", exitCode(err))
	}
	var lint struct {
		Count   int                    `json:"count"`
		Details []classification.Issue `json:"details"`
	}
	if err := json.Unmarshal(buf.Bytes(), &lint); err != nil {
		t.Fatalf("lint json: %v\n%s", err, buf)
	}
	if !slices.ContainsFunc(lint.Details, func(is classification.Issue) bool {
		return is.Code == classification.CodeStaleEntry && is.Path == "assign.gone"
	}) {

		t.Errorf("lint details = %+v", lint.Details)
	}
}

func TestClassificationCmd_SyncRefusesBrokenFile(t *testing.T) {
	dir := classificationProjectDir(t)
	withOutput(t, output.FormatTable)
	broken := "labels:\n  x: {}\n  x: {}\n"
	path := filepath.Join(dir, classification.FileName)
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, err := captureStdout(t, (&ClassificationSyncCmd{}).Run)
	if exitCode(err) != 1 || !strings.Contains(stdout, "sync refused") {
		t.Fatalf("exit=%d out=%q", exitCode(err), stdout)
	}
	if data, _ := os.ReadFile(path); string(data) != broken {
		t.Errorf("sync changed a file it refused")
	}
}

func TestClassificationCmd_OversizedFile(t *testing.T) {
	dir := classificationProjectDir(t)
	withOutput(t, output.FormatTable)
	big := make([]byte, classification.MaxFileSize+1)
	if err := os.WriteFile(filepath.Join(dir, classification.FileName), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&ClassificationLintCmd{}).Run(); err == nil || !strings.Contains(err.Error(), "the limit is") {
		t.Fatalf("err = %v", err)
	}
}

func TestClassificationCmd_NoProject(t *testing.T) {
	prev := projectPath
	projectPath = t.TempDir()
	t.Cleanup(func() { projectPath = prev })
	if err := (&ClassificationLintCmd{}).Run(); err == nil || !strings.Contains(err.Error(), "no project found") {
		t.Fatalf("err = %v", err)
	}
}

func TestReportClassificationValidation(t *testing.T) {
	dir := classificationProjectDir(t)
	mm, err := metamodel.Parse([]byte(classificationTestSchema))
	if err != nil {
		t.Fatal(err)
	}
	result := &projectsetup.ValidateResult{ProjectRoot: dir, Metamodel: mm}

	if got := reportClassificationValidation(result, false); got {
		t.Error("a project without classification.yaml must pass")
	}
	if err := os.WriteFile(filepath.Join(dir, classification.FileName), []byte("labels: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _ := captureStdout(t, func() error {
		if !reportClassificationValidation(result, false) {
			t.Error("missing entries must fail validate")
		}
		return nil
	})
	if !strings.Contains(stdout, "assign.person") {
		t.Errorf("output = %q", stdout)
	}
	if reportClassificationValidation(&projectsetup.ValidateResult{ProjectRoot: dir}, false) {
		t.Error("a broken schema is reported elsewhere; classification must not add an error")
	}
}

func TestClassificationShape(t *testing.T) {
	mm, err := metamodel.Parse([]byte(classificationTestSchema))
	if err != nil {
		t.Fatal(err)
	}
	shape := classificationShape(mm)
	if got := shape.Entities["person"]; !slices.Equal(got.Fields, []string{"name", "email", "body"}) || !got.OpaqueID {
		t.Errorf("person = %+v", got)
	}
	if got := shape.Entities["topic"]; got.OpaqueID {
		t.Errorf("manual-id topic must not be opaque: %+v", got)
	}
	rel := shape.Relations["follows"]
	if !slices.Equal(rel.Fields, []string{"since", "body"}) ||
		!slices.Equal(rel.Ends, []classification.Ends{{From: "person", To: "topic"}}) {

		t.Errorf("follows = %+v", rel)
	}
}

func TestPropertyOrder_WithoutDeclaredOrder(t *testing.T) {
	props := map[string]metamodel.PropertyDef{"b": {}, "a": {}, "c": {}}
	if got := propertyOrder([]string{"c", "ghost"}, props); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Errorf("propertyOrder = %v", got)
	}
}

func TestClassificationRenames(t *testing.T) {
	got, err := classificationRenames(fstest.MapFS{})
	if err != nil || len(got) != 0 {
		t.Errorf("no migrations dir: %v, %v", got, err)
	}
	_, err = classificationRenames(fstest.MapFS{
		"migrations/20260919143022-bad.yaml": {Data: []byte("steps: [")},
	})
	if err == nil {
		t.Error("a broken migration file must surface")
	}
}

const classificationTestFile = `labels:
  name: { role: direct-identifier }
  topic-interest: { role: attribute }
  interested:
    when: { scope: subject, all_of: ["@direct-identifier", topic-interest] }
assign:
  person:
    name: [name]
    email: none
    body: none
  topic:
    title: [topic-interest]
    body: none
`

func TestClassificationCmd_Report(t *testing.T) {
	dir := classificationProjectDir(t)
	withOutput(t, output.FormatTable)

	stdout, err := captureStdout(t, (&ClassificationReportCmd{}).Run)
	if err != nil || !strings.Contains(stdout, "no classification.yaml") {
		t.Fatalf("report without file: err=%v out=%q", err, stdout)
	}

	path := filepath.Join(dir, classification.FileName)
	if writeErr := os.WriteFile(path, []byte(classificationTestFile), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	stdout, err = captureStdout(t, (&ClassificationReportCmd{}).Run)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"person: person.name has name (direct-identifier)",
		"reaches topic via -> follows topic",
		"derived interested from person.name, topic.title",
		"follows (subject link)",
		"since                missing",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("report lacks %q:\n%s", want, stdout)
		}
	}

	buf := withOutput(t, output.FormatJSON)
	if runErr := (&ClassificationReportCmd{}).Run(); runErr != nil {
		t.Fatal(runErr)
	}
	var res struct {
		Details classification.Report `json:"details"`
	}
	if jsonErr := json.Unmarshal(buf.Bytes(), &res); jsonErr != nil {
		t.Fatalf("report json: %v\n%s", jsonErr, buf)
	}
	if len(res.Details.Subjects) != 1 || res.Details.Subjects[0].Subject != "person" {
		t.Errorf("subjects = %+v", res.Details.Subjects)
	}

	if writeErr := os.WriteFile(path, []byte("labels: [\n"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	withOutput(t, output.FormatTable)
	stdout, err = captureStdout(t, (&ClassificationReportCmd{}).Run)
	if exitCode(err) != 1 || !strings.Contains(stdout, "report refused") {
		t.Errorf("broken file: exit=%d out=%q", exitCode(err), stdout)
	}
}
