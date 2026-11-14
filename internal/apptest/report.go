package apptest

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Report is the outcome of a run.
type Report struct {
	// Seed is the run seed. Passing it back with [Options.Seed] reproduces
	// every generated value and every case's starting clock.
	Seed uint64 `json:"seed"`
	// Cases holds one result per case run, in run order. A file that fails
	// before its cases run (a Lua error at the top level, no cases) has one
	// result with an empty Case.
	Cases  []CaseResult `json:"cases"`
	Passed int          `json:"passed"`
	Failed int          `json:"failed"`
}

// OK reports whether every case passed.
func (r *Report) OK() bool { return r.Failed == 0 }

// CaseResult is the outcome of one case.
type CaseResult struct {
	// File is the test file, relative to the project root, with forward
	// slashes.
	File string `json:"file"`
	// Case is the name given to t.case; empty for a file-level failure.
	Case string `json:"case,omitempty"`
	OK   bool   `json:"ok"`
	// Location is file:line of the failure: the assertion that failed, or
	// the line a Lua error names. Empty when the case passed or the error
	// names no line.
	Location string `json:"location,omitempty"`
	// Message says why the case failed.
	Message  string        `json:"message,omitempty"`
	Duration time.Duration `json:"duration_ns"`
}

// add records a result and updates the counts.
func (r *Report) add(c CaseResult) {
	r.Cases = append(r.Cases, c)
	if c.OK {
		r.Passed++
	} else {
		r.Failed++
	}
}

// WriteText writes one line per case and a summary that names the seed.
func (r *Report) WriteText(w io.Writer) error {
	for _, c := range r.Cases {
		name := c.File
		if c.Case != "" {
			name += ": " + c.Case
		}
		if c.OK {
			if _, err := fmt.Fprintf(w, "ok      %s (%s)\n", name, c.Duration.Round(time.Millisecond)); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "not ok  %s\n", name); err != nil {
			return err
		}
		where := c.Location
		if where != "" {
			where += ": "
		}
		if _, err := fmt.Fprintf(w, "        %s%s\n", where, c.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%d passed, %d failed (seed %d)\n", r.Passed, r.Failed, r.Seed)
	return err
}

// WriteJSON writes the report as one JSON document.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
