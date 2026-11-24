package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
)

const baselinePath = "tools/commentbody/baseline.json"

func main() {
	update := flag.Bool("update", false, "rewrite the baseline to the current counts")
	flag.Parse()
	if err := run(".", *update); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, update bool) error {
	found, err := scanTree(root)
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for file, list := range found {
		counts[file] = len(list)
	}
	if update {
		return writeBaseline(root+"/"+baselinePath, counts)
	}
	baseline, err := readBaseline(root + "/" + baselinePath)
	if err != nil {
		return err
	}
	for _, note := range improvements(counts, baseline) {
		fmt.Fprintln(os.Stderr, note)
	}
	problems := compare(counts, baseline, found)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		return fmt.Errorf("commentbody: %d file(s) off baseline", len(problems))
	}
	return nil
}

func compare(counts, baseline map[string]int, found map[string][]finding) []string {
	var problems []string
	for _, file := range sortedKeys(counts) {
		if counts[file] > baseline[file] {
			problems = append(problems, fmt.Sprintf("%s: %d findings, baseline allows %d", file, counts[file], baseline[file]))
			for _, f := range found[file] {
				problems = append(problems, fmt.Sprintf("  %s: %s", f.pos, f.kind))
			}
		}
	}
	return problems
}

func improvements(counts, baseline map[string]int) []string {
	var notes []string
	for _, file := range sortedKeys(baseline) {
		if counts[file] < baseline[file] {
			notes = append(notes, fmt.Sprintf(
				"note: %s has %d findings, baseline says %d; `just comment-body-update` locks in the gain",
				file, counts[file], baseline[file]))
		}
	}
	return notes
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readBaseline(path string) (map[string]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	baseline := map[string]int{}
	return baseline, json.Unmarshal(data, &baseline)
}

func writeBaseline(path string, counts map[string]int) error {
	data, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
