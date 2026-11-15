package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBench(t *testing.T, name, cpu string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	body := "goos: linux\ncpu: " + cpu + "\nBenchmarkX/n=1-4 \t10\t100 ns/op\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCompareBench_Notes(t *testing.T) {
	cpuA := writeBench(t, "a.txt", "CPU A")
	cpuB := writeBench(t, "b.txt", "CPU B")
	tests := []struct {
		name string
		in   benchInput
		want string
	}{
		{"not run", benchInput{}, "Not run."},
		{"failed", benchInput{note: "The benchmarks failed."}, "The benchmarks failed."},
		{"no baseline", benchInput{old: filepath.Join(t.TempDir(), "missing"), new: cpuA}, "No baseline"},
		{"other cpu", benchInput{old: cpuA, new: cpuB}, `ran on "CPU A" and this run on "CPU B"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, note, deltas := compareBench(tc.in)
			if text != "" || deltas != nil || !strings.Contains(note, tc.want) {
				t.Fatalf("got text %q, note %q, deltas %v; want a note containing %q", text, note, deltas, tc.want)
			}
		})
	}
}

func TestParseScales(t *testing.T) {
	got, err := parseScales("1, 5", 1)
	if err != nil || len(got) != 2 || got[1] != 5 {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []string{"5", "1,1", "1,x", "1,0", "1,11"} {
		if _, err := parseScales(bad, 1); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestWithSearchPath(t *testing.T) {
	got, err := withSearchPath("postgres://u@h/db?sslmode=disable", "s,public")
	if err != nil || !strings.Contains(got, "search_path=s%2Cpublic") || !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("got %q, %v", got, err)
	}
	if got, _ := withSearchPath("host=h dbname=db", "s"); got != "host=h dbname=db search_path=s" {
		t.Fatalf("keyword form: %q", got)
	}
	_, err = withSearchPath("postgres://u:secret@h:bad/db", "s")
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("bad URL: %v", err)
	}
}
