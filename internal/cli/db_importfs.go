package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Sourcehaven-BV/rela/internal/fsimport"
)

// printImportReport writes what an import did, or found, to w.
//
// The skipped files are always listed in full: a file left behind is the
// thing an operator must be able to check, and a count would hide which.
func printImportReport(w io.Writer, rep *fsimport.Report, runErr error) {
	if rep == nil {
		return
	}
	printImportFindings(w, rep)
	if runErr != nil {
		return
	}
	p := reportPrinter(w)
	p("\nImported %s into %s:", rep.Source, rep.Target)
	p("  %s", importedCounts(rep))
	p("  %d project files; %d date or time values stored in database form", rep.Files, rep.Normalized)
	p("\nNext steps:")
	p("  - Open the new project with the SQLite build (rela-sqlite, rela-server-sqlite).")
	p("  - Run 'rela-sqlite analyze' there. The import copies rows as they are and does not")
	p("    validate them against the schema.")
	p("  - If you use a systemd credential for secrets, run 'rela secrets credential-name' in the")
	p("    new directory: the credential name depends on the project path.")
	p("  - The new project is not a git repository; run 'git init' there if you want one.")
	p("    .rela/ is in its .gitignore, so the database is never committed.")
	p("  - History starts now: the first start records a version of every row, and search")
	p("    builds its index.")
	if rep.GitCrypt {
		p("  - The source encrypts files with git-crypt. The new project holds them UNENCRYPTED:")
		p("    data in the database, and project files that a new git repository commits in")
		p("    clear unless you set up git-crypt there first. Protect the directory accordingly.")
	}
}

// importedCounts is the one-line count of what an import copied.
func importedCounts(rep *fsimport.Report) string {
	return fmt.Sprintf("%d entities, %d relations, %d attachments, %d comments, %d state keys",
		rep.Entities, rep.Relations, rep.Attachments, rep.Comments, rep.StateKeys)
}

// reportPrinter prints one report line. Paths and reasons name source files,
// whose names the source's author chose; a control character in one could
// rewrite earlier report lines, so string arguments are made printable.
func reportPrinter(w io.Writer) func(format string, args ...any) {
	return func(format string, args ...any) {
		for i, a := range args {
			if s, ok := a.(string); ok {
				args[i] = printable(s)
			}
		}
		_, _ = fmt.Fprintf(w, format+"\n", args...)
	}
}

// printImportFindings prints what both imports report besides the counts:
// problems, files not copied, pending deletes and warnings.
func printImportFindings(w io.Writer, rep *fsimport.Report) {
	if rep == nil {
		return
	}
	p := reportPrinter(w)
	if len(rep.Errors) > 0 {
		p("\nProblems (%d); the import did not complete:", len(rep.Errors))
		for _, e := range rep.Errors {
			p("  - %s", e)
		}
	}
	if len(rep.Skipped) > 0 {
		p("\nNot copied (%d):", len(rep.Skipped))
		for _, s := range rep.Skipped {
			p("  - %s: %s", s.Path, s.Reason)
		}
	}
	if len(rep.PendingDeletes) > 0 {
		p("\nDeleted in the source with undo still pending, not copied: %s",
			strings.Join(rep.PendingDeletes, ", "))
	}
	if len(rep.Warnings) > 0 {
		p("\nWarnings:")
		for _, wn := range rep.Warnings {
			p("  - %s", wn)
		}
	}
}

// printable replaces control characters with U+FFFD.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return unicode.ReplacementChar
		}
		return r
	}, s)
}
