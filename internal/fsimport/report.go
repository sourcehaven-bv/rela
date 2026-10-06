package fsimport

import (
	"fmt"
	"maps"
	"slices"
)

// Report describes one run.
type Report struct {
	Source string
	Target string

	Entities    int
	Relations   int
	Attachments int
	Comments    int
	StateKeys   int
	Files       int

	// Normalized counts property values converted to the database form
	// (dates and timestamps; see normalizeProps).
	Normalized int

	// Skipped lists every source file the import did not copy, with the
	// reason, sorted by path.
	Skipped []Skip

	// PendingDeletes lists the ids of entities deleted in the source but
	// still undoable there. They are not copied.
	PendingDeletes []string

	// Errors lists every problem that failed the run. Empty on success.
	Errors []string

	// Warnings lists problems that did not fail the run.
	Warnings []string

	// GitCrypt reports that the source's .gitattributes encrypts data paths,
	// which the target stores unencrypted.
	GitCrypt bool
}

// Skip is a source file the import did not copy.
type Skip struct {
	Path   string
	Reason string
}

func newReport(source, target string) *Report {
	return &Report{Source: source, Target: target}
}

func (r *Report) skip(path, format string, args ...any) {
	r.Skipped = append(r.Skipped, Skip{Path: path, Reason: fmt.Sprintf(format, args...)})
}

func (r *Report) fail(format string, args ...any) {
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

func (r *Report) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// sortSkipped orders the skip list by path so the report is stable.
func (r *Report) sortSkipped() {
	slices.SortStableFunc(r.Skipped, func(a, b Skip) int {
		switch {
		case a.Path < b.Path:
			return -1
		case a.Path > b.Path:
			return 1
		}
		return 0
	})
}

// summary is the one-line description used for the audit record.
func (r *Report) summary() string {
	return fmt.Sprintf("imported %s: %d entities, %d relations, %d attachments, %d comments, "+
		"%d state keys, %d files; %d values normalized, %d files not copied",
		r.Source, r.Entities, r.Relations, r.Attachments, r.Comments,
		r.StateKeys, r.Files, r.Normalized, len(r.Skipped))
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
