package cli

import (
	stderrors "errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/classification"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// ClassificationCmd groups the classification.yaml commands. They read only
// schema.yaml, classification.yaml and migrations/, and open no store, so
// they run in CI on a checkout without data.
type ClassificationCmd struct {
	Sync   ClassificationSyncCmd   `cmd:"" help:"Create or update classification.yaml from the schema."`
	Lint   ClassificationLintCmd   `cmd:"" help:"Check classification.yaml against the schema."`
	Report ClassificationReportCmd `cmd:"" help:"Show labels, inferred subjects and derived labels."`
}

// ClassificationSyncCmd is `rela classification sync`.
type ClassificationSyncCmd struct {
	DryRun bool `help:"Report what would change without writing the file."`
}

// ClassificationLintCmd is `rela classification lint`.
type ClassificationLintCmd struct{}

// ClassificationReportCmd is `rela classification report`.
type ClassificationReportCmd struct{}

// classificationProject is what every classification command reads.
type classificationProject struct {
	root   string
	path   string
	data   []byte
	exists bool
	meta   *metamodel.Metamodel
	fs     storage.FS
}

func loadClassificationProject() (*classificationProject, error) {
	fsys := storage.NewSafeFS(storage.NewOsFS())
	pctx, err := project.Discover(projectPath, fsys)
	if err != nil {
		return nil, stderrors.New("no project found: run 'rela init' to create one")
	}
	mm, _, err := metamodel.Load(pctx.SchemaPath, fsys)
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	p := &classificationProject{
		root: pctx.Root,
		path: filepath.Join(pctx.Root, classification.FileName),
		meta: mm,
		fs:   fsys,
	}
	if p.data, p.exists, err = readClassificationFile(p.path); err != nil {
		return nil, err
	}
	return p, nil
}

// readClassificationFile reads the overlay. A missing file is not an error:
// exists is false. The size is checked before reading so an oversized file
// is never loaded.
func readClassificationFile(path string) (data []byte, exists bool, err error) {
	// #nosec G703 -- path is <discovered project root>/classification.yaml, a
	// fixed name under the operator's own project, not request input.
	info, err := os.Stat(path)
	switch {
	case stderrors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("read %s: %w", classification.FileName, err)
	case info.Size() > classification.MaxFileSize:
		return nil, false, fmt.Errorf("%s is %d bytes; the limit is %d",
			classification.FileName, info.Size(), classification.MaxFileSize)
	}
	if data, err = os.ReadFile(path); err != nil { // #nosec G703 -- see os.Stat above
		return nil, false, fmt.Errorf("read %s: %w", classification.FileName, err)
	}
	return data, true, nil
}

// Run executes `rela classification sync`.
func (c *ClassificationSyncCmd) Run() error {
	p, err := loadClassificationProject()
	if err != nil {
		return err
	}
	renames, err := classificationRenames(os.DirFS(p.root))
	if err != nil {
		return err
	}
	res, issues := classification.Sync(p.data, classificationShape(p.meta), renames)
	if len(issues) > 0 {
		writeClassificationIssues("sync refused: fix these first", issues)
		return errors.NewExitError(1)
	}
	if res.Changed && !c.DryRun {
		if err := p.fs.WriteFile(p.path, res.Output, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", classification.FileName, err)
		}
	}
	writeSyncResult(res, c.DryRun, p.exists)
	return nil
}

// Run executes `rela classification lint`.
func (c *ClassificationLintCmd) Run() error {
	p, err := loadClassificationProject()
	if err != nil {
		return err
	}
	if !p.exists {
		return writeNoClassificationFile()
	}
	issues := lintClassification(p.data, p.meta)
	if len(issues) == 0 {
		msg := classification.FileName + " matches the schema"
		if out.Format == output.FormatJSON {
			return out.WriteAnalysisResult(output.AnalysisResult{Status: "success", Message: msg})
		}
		if !quiet {
			fmt.Println("✓ " + msg)
		}
		return nil
	}
	writeClassificationIssues(fmt.Sprintf("%d classification issue(s)", len(issues)), issues)
	return errors.NewExitError(1)
}

// writeNoClassificationFile reports an absent file. It is not an error: the
// file is optional.
func writeNoClassificationFile() error {
	msg := fmt.Sprintf("no %s; run `rela classification sync` to create one", classification.FileName)
	if out.Format == output.FormatJSON {
		return out.WriteAnalysisResult(output.AnalysisResult{Status: "success", Message: msg})
	}
	if !quiet {
		fmt.Println(msg)
	}
	return nil
}

// lintClassification parses and lints; a file that does not parse is not
// linted, because lint would report its unread parts as missing.
func lintClassification(data []byte, mm *metamodel.Metamodel) []classification.Issue {
	f, issues := classification.Parse(data)
	if len(issues) > 0 {
		return issues
	}
	return classification.Lint(f, classificationShape(mm))
}

func writeClassificationIssues(message string, issues []classification.Issue) {
	if out.Format == output.FormatJSON {
		_ = out.WriteAnalysisResult(output.AnalysisResult{
			Status: "error", Message: message, Count: len(issues), Details: issues,
		})
		return
	}
	for _, is := range issues {
		fmt.Println("  ✗ " + is.String())
	}
	fmt.Println(message)
}

type syncResultJSON struct {
	Written   bool     `json:"written"`
	Added     []string `json:"added"`
	Moved     []string `json:"moved"`
	Stale     []string `json:"stale"`
	Conflicts []string `json:"conflicts"`
}

func writeSyncResult(res classification.SyncResult, dryRun, existed bool) {
	var msg string
	switch {
	case !res.Changed:
		msg = classification.FileName + " is up to date"
	case dryRun:
		msg = "dry run: " + classification.FileName + " would change"
	case !existed:
		msg = "created " + classification.FileName
	default:
		msg = "updated " + classification.FileName
	}
	if out.Format == output.FormatJSON {
		status := "success"
		if len(res.Stale)+len(res.Conflicts) > 0 {
			status = "warning"
		}
		_ = out.WriteAnalysisResult(output.AnalysisResult{
			Status: status, Message: msg, Count: len(res.Added) + len(res.Moved),
			Details: syncResultJSON{
				Written: res.Changed && !dryRun,
				Added:   orEmpty(res.Added), Moved: orEmpty(res.Moved),
				Stale: orEmpty(res.Stale), Conflicts: orEmpty(res.Conflicts),
			},
		})
		return
	}
	for _, group := range []struct {
		title string
		items []string
	}{
		{"added as needs-review", res.Added},
		{"moved across a rename", res.Moved},
		{"stale (not in the schema; remove by hand)", res.Stale},
		{"rename conflicts (resolve by hand)", res.Conflicts},
	} {
		if len(group.items) == 0 {
			continue
		}
		fmt.Printf("%s:\n", group.title)
		for _, item := range group.items {
			fmt.Printf("  %s\n", item)
		}
	}
	fmt.Println(msg)
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// reportClassificationValidation lints classification.yaml as part of
// `rela validate` when the file exists. A project without the file is not
// asked to have one.
func reportClassificationValidation(result *projectsetup.ValidateResult, hasErrors bool) bool {
	if result.Metamodel == nil {
		return hasErrors
	}
	data, exists, err := readClassificationFile(filepath.Join(result.ProjectRoot, classification.FileName))
	if err != nil {
		fmt.Printf("  ✗ %v\n", err)
		return true
	}
	if !exists {
		return hasErrors
	}
	issues := lintClassification(data, result.Metamodel)
	if len(issues) == 0 {
		if !quiet {
			fmt.Printf("  ✓ %s is valid\n", classification.FileName)
		}
		return hasErrors
	}
	for _, is := range issues {
		fmt.Printf("  ✗ %s\n", is)
	}
	return true
}

// Run executes `rela classification report`. It reports on a file with lint
// issues too (a missing entry shows as missing), but not on one that does
// not parse.
func (c *ClassificationReportCmd) Run() error {
	p, err := loadClassificationProject()
	if err != nil {
		return err
	}
	if !p.exists {
		return writeNoClassificationFile()
	}
	f, issues := classification.Parse(p.data)
	if len(issues) > 0 {
		writeClassificationIssues("report refused: fix these first", issues)
		return errors.NewExitError(1)
	}
	rep := classification.BuildReport(f, classificationShape(p.meta))
	if out.Format == output.FormatJSON {
		return out.WriteAnalysisResult(output.AnalysisResult{
			Status: "success", Message: "classification report", Count: len(rep.Subjects), Details: rep,
		})
	}
	writeReportText(rep)
	return nil
}

func writeReportText(rep classification.Report) {
	fmt.Println("Subjects")
	if len(rep.Subjects) == 0 {
		fmt.Println("  none: no type has a direct-identifier field")
	}
	reasons := map[string]string{}
	manualIDs := map[string]bool{}
	for _, t := range rep.Types {
		reasons[t.Type] = t.Subject
		manualIDs[t.Type] = t.IDMayIdentify
	}
	for _, s := range rep.Subjects {
		fmt.Printf("  %s: %s\n", s.Subject, reasons[s.Subject])
		if manualIDs[s.Subject] {
			fmt.Println("    ids are not opaque and may identify a person")
		}
		for _, r := range s.Reached {
			fmt.Printf("    reaches %s via %s\n", r.Type, hopPath(r.Path))
		}
		if s.Truncated {
			fmt.Printf("    (truncated at %d types)\n", classification.MaxProfileTypes)
		}
		writeDerivations("    ", s.Derived)
	}
	fmt.Println("\nEntity types")
	for _, t := range rep.Types {
		fmt.Printf("  %s\n", t.Type)
		writeFieldReports(t.Fields)
		writeDerivations("    ", t.Derived)
	}
	fmt.Println("\nRelation types")
	for _, r := range rep.Relations {
		link := ""
		if r.SubjectLink {
			link = " (subject link)"
		}
		fmt.Printf("  %s%s\n", r.Relation, link)
		writeFieldReports(r.Fields)
		writeDerivations("    ", r.Derived)
	}
}

func writeFieldReports(fields []classification.FieldReport) {
	for _, fr := range fields {
		value := fr.State
		if len(fr.Labels) > 0 {
			value = strings.Join(fr.Labels, ", ")
		}
		fmt.Printf("    %-20s %s\n", fr.Field, value)
	}
}

func writeDerivations(indent string, ds []classification.Derivation) {
	for _, d := range ds {
		fmt.Printf("%sderived %s from %s\n", indent, d.Label, strings.Join(d.Fields, ", "))
	}
}

func hopPath(path []classification.Hop) string {
	parts := make([]string, 0, len(path))
	for _, h := range path {
		arrow := "<-"
		if h.Outgoing {
			arrow = "->"
		}
		parts = append(parts, fmt.Sprintf("%s %s %s", arrow, h.Relation, h.Type))
	}
	return strings.Join(parts, " ")
}
