package scalerun

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// BenchDelta is one row of a benchstat comparison.
type BenchDelta struct {
	// Name is the benchmark name, prefixed with its package when benchstat
	// printed one.
	Name string
	Unit string
	// Delta is the relative change, new/old - 1. Zero when Significant is
	// false.
	Delta float64
	// Significant is false when benchstat printed "~": the change is within
	// noise at its default confidence.
	Significant bool
}

// ParseBenchstatCSV reads the output of `benchstat -format csv OLD NEW` and
// returns the time-per-op rows. Rows for other units (B/op, allocs/op) and
// the geomean summary are skipped: allocation counts change with intended
// work, and the gate is about time.
func ParseBenchstatCSV(r io.Reader) ([]BenchDelta, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	var out []BenchDelta
	pkg, unit := "", ""
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("benchstat csv: %w", err)
		}
		if len(rec) == 1 {
			// goos:/goarch:/pkg:/cpu: lines come through as one field.
			if p, ok := strings.CutPrefix(rec[0], "pkg: "); ok {
				pkg = p
			}
			continue
		}
		if rec[0] == "" {
			// A header row: ",sec/op,CI,sec/op,CI,vs base,P" names the unit.
			if len(rec) > 1 && rec[1] != "" && !strings.Contains(rec[1], ".") {
				unit = rec[1]
			}
			continue
		}
		if unit != "sec/op" || rec[0] == "geomean" || len(rec) < 6 {
			continue
		}
		d := BenchDelta{Name: rec[0], Unit: unit}
		if pkg != "" {
			d.Name = pkg + "." + rec[0]
		}
		vs := strings.TrimSpace(rec[5])
		if pct, ok := strings.CutSuffix(vs, "%"); ok {
			v, err := strconv.ParseFloat(pct, 64)
			if err != nil {
				return nil, fmt.Errorf("benchstat csv: %s: delta %q: %w", d.Name, vs, err)
			}
			d.Delta, d.Significant = v/100, true
		}
		out = append(out, d)
	}
	return out, nil
}
