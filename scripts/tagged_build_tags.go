//go:build ignore

// Command tagged_build_tags prints the set of opt-in build tags that appear
// on Go files, one per line, sorted.
//
// It exists because build constraints cannot be parsed correctly with grep and
// sed. The first draft of check-tagged-tests.sh tried, and missed a constraint
// written with a TAB (`//go:build<TAB>e2e`) — which Go honours — so a rotted
// file produced a cheerful "nothing to compile" and exit 0. That is the exact
// failure the guard exists to prevent, so discovery now uses the toolchain's
// own parser, go/build/constraint, and is correct by construction for:
//
//   - any whitespace after the directive, including tabs
//   - parenthesised and nested expressions
//   - negation SCOPE, so `!(a || b)` yields nothing rather than a and b
//   - the legacy `// +build` syntax (still honoured when no //go:build is
//     present) and the rule that //go:build wins when both appear
//   - constraints that must precede the package clause, and only blank or
//     comment lines before them
//
// Note: a //go:build line inside a /* */ block comment is reported as a tag,
// where Go would reject the file outright with "misplaced //go:build comment".
// That is a harmless false positive — it costs one extra vet run and can never
// hide a break — and the compiler already reports the real problem.
//
// Run it via scripts/check-tagged-tests.sh; it is not meant to be used alone.
//
// Usage:
//
//	go run scripts/tagged_build_tags.go [-root DIR] [-pattern GLOB]
package main

import (
	"flag"
	"fmt"
	"go/build/constraint"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// platformTags are constraints that select a platform or toolchain rather than
// opting a file in. `go vet -tags windows` does NOT check the Windows build: it
// type-checks against the host platform with an extra tag, which collides with
// the real GOOS and reports spurious redeclaration errors. Cross-platform files
// are checked with `GOOS=<x> go vet` instead, which is a separate concern.
var platformTags = map[string]bool{
	// GOOS
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true, "js": true,
	"linux": true, "nacl": true, "netbsd": true, "openbsd": true,
	"plan9": true, "solaris": true, "wasip1": true, "windows": true,
	// GOARCH
	"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true,
	"mips": true, "mips64": true, "mips64le": true, "mips64p32": true,
	"mips64p32le": true, "mipsle": true, "ppc": true, "ppc64": true,
	"ppc64le": true, "riscv": true, "riscv64": true, "s390x": true,
	"sparc": true, "sparc64": true, "wasm": true,
	// Toolchain-derived: set by the build, never something to pass via -tags.
	"cgo": true, "race": true, "msan": true, "asan": true, "gc": true,
	"gccgo": true, "unix": true, "purego": true, "boringcrypto": true,
	// `ignore` conventionally means "never build this file" — including this
	// one. Vetting it is meaningless.
	"ignore": true,
}

// skipTag reports whether tag selects a platform/toolchain rather than opting a
// file in. Release tags (go1.24) are derived from the toolchain version, so
// passing them via -tags is meaningless too.
func skipTag(tag string) bool {
	if platformTags[tag] {
		return true
	}
	return strings.HasPrefix(tag, "go1.")
}

// collect walks a parsed constraint expression and records every tag that can
// make the file build, i.e. every operand appearing under an EVEN number of
// negations.
//
// Polarity is the whole point. A file behind `!postgres` builds by DEFAULT, so
// `postgres` is not an opt-in tag and vetting it would EXCLUDE that file. The
// naive text approach got this wrong for `!(a || b)`, because stripping parens
// orphans the `!` from the group it negates.
func collect(e constraint.Expr, negated bool, out map[string]bool) {
	switch x := e.(type) {
	case *constraint.TagExpr:
		if !negated {
			out[x.Tag] = true
		}
	case *constraint.NotExpr:
		collect(x.X, !negated, out)
	case *constraint.AndExpr:
		collect(x.X, negated, out)
		collect(x.Y, negated, out)
	case *constraint.OrExpr:
		collect(x.X, negated, out)
		collect(x.Y, negated, out)
	}
}

// fileTags returns the opt-in tags constraining a single file.
//
// Precedence follows the go command: if any //go:build line is present it wins
// outright and every // +build line is ignored. Only the header — the region
// before the package clause — is considered, because a constraint after it is
// just a comment.
func fileTags(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var goBuild constraint.Expr
	var plusBuild []constraint.Expr

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))

		// The header ends at the package clause. Stop there: Go ignores
		// anything later, and so must we, or a commented-out constraint deep in
		// a file would invent a tag.
		if strings.HasPrefix(line, "package ") || line == "package" {
			break
		}

		if constraint.IsGoBuild(line) {
			expr, err := constraint.Parse(line)
			if err != nil {
				// A malformed constraint is not this tool's to report — the
				// compiler will. Skipping it cannot hide rot, because a file
				// Go cannot parse does not build either.
				continue
			}
			goBuild = expr
			// Keep scanning: //go:build must precede package, and taking the
			// last one matches how a stray earlier line would be overridden.
			continue
		}

		if constraint.IsPlusBuild(line) {
			if expr, err := constraint.Parse(line); err == nil {
				plusBuild = append(plusBuild, expr)
			}
		}
	}

	out := map[string]bool{}
	switch {
	case goBuild != nil:
		collect(goBuild, false, out)
	default:
		// Multiple // +build lines are ANDed. Polarity is per-operand, so
		// collecting each independently is right.
		for _, expr := range plusBuild {
			collect(expr, false, out)
		}
	}
	return out, nil
}

func main() {
	root := flag.String("root", ".", "repository root to scan")
	pattern := flag.String("pattern", "*_test.go", "file glob to scan")
	flag.Parse()

	info, err := os.Stat(*root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "ERROR: root not a directory: %s\n", *root)
		os.Exit(2)
	}

	all := map[string]bool{}

	err = filepath.WalkDir(*root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Third-party and generated trees carry tags that are not ours to
			// keep compiling.
			switch d.Name() {
			case "vendor", "node_modules", ".git", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		if ok, _ := filepath.Match(*pattern, d.Name()); !ok {
			return nil
		}
		tags, err := fileTags(path)
		if err != nil {
			return err
		}
		for tag := range tags {
			all[tag] = true
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: scanning %s: %v\n", *root, err)
		os.Exit(2)
	}

	var tags []string
	for tag := range all {
		if !skipTag(tag) {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)

	for _, tag := range tags {
		fmt.Println(tag)
	}
}
