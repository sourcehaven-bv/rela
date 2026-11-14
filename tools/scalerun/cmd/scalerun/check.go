package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/tools/scalerun"
)

// check writes the report and returns the number of findings.
func check(cfg *scalerun.Config, resultsPath, reportPath string, b benchInput) (int, error) {
	raw, err := os.ReadFile(resultsPath)
	if err != nil {
		return 0, err
	}
	var res scalerun.Results
	if err = json.Unmarshal(raw, &res); err != nil {
		return 0, fmt.Errorf("%s: %w", resultsPath, err)
	}
	in := scalerun.ReportInput{Config: cfg, Results: &res, RunURL: os.Getenv("SCALERUN_RUN_URL")}
	var deltas []scalerun.BenchDelta
	in.Benchstat, in.BenchNote, deltas = compareBench(b)
	in.Findings = scalerun.Check(cfg, &res, deltas)
	if len(res.Scales) == 0 {
		in.Findings = append(in.Findings, scalerun.Finding{Kind: "error", Message: "the run measured no scale"})
	}
	if reportPath == "" {
		reportPath = filepath.Join(filepath.Dir(resultsPath), "report.md")
	}
	f, err := os.Create(reportPath)
	if err != nil {
		return 0, err
	}
	werr := scalerun.WriteReport(f, in)
	if err := errors.Join(werr, f.Close()); err != nil {
		return 0, err
	}
	for _, fd := range in.Findings {
		fmt.Printf("%s: %s\n", fd.Kind, fd.Message)
	}
	fmt.Printf("report: %s\n", reportPath)
	return len(in.Findings), nil
}

// benchInput names the benchmark files check compares.
type benchInput struct {
	old, new string
	// note explains why new is missing, e.g. that the benchmarks failed.
	note string
}

// compareBench compares the benchmark files. It returns benchstat's table,
// a note for the report when there is nothing to gate on, and the deltas
// to gate on.
//
// A baseline that cannot be compared is a note, not a failure: only a
// passing run replaces the baseline, so failing on an unusable one would
// fail every later run too. The note is in the report, so the gap shows.
func compareBench(b benchInput) (text, note string, deltas []scalerun.BenchDelta) {
	switch {
	case b.new == "" && b.note != "":
		return "", b.note, nil
	case b.new == "":
		return "", "Not run.", nil
	case !exists(b.old):
		return "", "No baseline to compare with: this is the first run, no earlier run passed, " +
			"or the baseline was reset. This run's results become the baseline if it passes.", nil
	}
	oldCPU, newCPU := cpuLine(b.old), cpuLine(b.new)
	if oldCPU != newCPU {
		return "", fmt.Sprintf("Not compared: the baseline ran on %q and this run on %q, "+
			"so a difference would measure the hardware. This run becomes the baseline if it passes.",
			oldCPU, newCPU), nil
	}
	text, csv, err := benchstat(b.old, b.new)
	if err != nil {
		return "", fmt.Sprintf("Not compared: %v. This run becomes the baseline if it passes.", err), nil
	}
	deltas, err = scalerun.ParseBenchstatCSV(bytes.NewReader(csv))
	if err != nil {
		return text, fmt.Sprintf("Not gated: reading benchstat's CSV: %v.", err), nil
	}
	if len(deltas) == 0 {
		return text, "Not gated: no benchmark appears in both runs (renamed benchmarks, " +
			"or a GOMAXPROCS suffix that differs). This run becomes the baseline if it passes.", nil
	}
	return text, "", deltas
}

// cpuLine returns the "cpu:" header go test writes, or "" if there is none.
func cpuLine(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(raw)) {
		if v, ok := strings.CutPrefix(line, "cpu: "); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// benchstat runs benchstat twice: text for the report, csv for the gate.
func benchstat(oldPath, newPath string) (text string, csv []byte, err error) {
	bin, err := exec.LookPath("benchstat")
	if err != nil {
		return "", nil, errors.New("benchstat is not on PATH; go install golang.org/x/perf/cmd/benchstat@latest")
	}
	// Labels instead of file paths keep the table readable.
	args := []string{"old=" + oldPath, "new=" + newPath}
	t, err := exec.Command(bin, args...).Output() //nolint:noctx // fixed tool, our files
	if err != nil {
		return "", nil, fmt.Errorf("benchstat: %w", err)
	}
	c, err := exec.Command(bin, append([]string{"-format", "csv"}, args...)...).Output() //nolint:noctx // same
	if err != nil {
		return "", nil, fmt.Errorf("benchstat -format csv: %w", err)
	}
	return strings.TrimSpace(string(t)), c, nil
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}
