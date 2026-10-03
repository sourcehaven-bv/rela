package agentrules

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const rulesDir = ".claude/rules"

// TestRulePathsMatchFiles fails when a rule file has no paths, or when one of
// its globs matches no tracked-looking file in the repository.
func TestRulePathsMatchFiles(t *testing.T) {
	root := repoRoot(t)
	files := repoFiles(t, root)
	rules, err := filepath.Glob(filepath.Join(root, rulesDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) == 0 {
		t.Fatalf("no rule files in %s", rulesDir)
	}
	for _, rule := range rules {
		t.Run(filepath.Base(rule), func(t *testing.T) {
			globs := rulePaths(t, rule)
			if len(globs) == 0 {
				t.Fatalf("no paths: frontmatter; the rule would load for every file")
			}
			for _, g := range globs {
				re, err := globRegexp(g)
				if err != nil {
					t.Errorf("%q: %v", g, err)
					continue
				}
				if !slices.ContainsFunc(files, re.MatchString) {
					t.Errorf("%q matches no file; update it after a move or rename", g)
				}
			}
		})
	}
}

func TestGlobRegexp(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"internal/mail/**", "internal/mail/outbox.go", true},
		{"internal/mail/**", "internal/mail/smtp/conn.go", true},
		{"internal/mail/**", "internal/mailrender/render.go", false},
		{"internal/lua/mail*.go", "internal/lua/mailrender.go", true},
		{"internal/lua/mail*.go", "internal/lua/sub/mail.go", false},
		{"**/*.go", "main.go", true},
		{"**/*.go", "a/b/c.go", true},
		{"src/*.{ts,tsx}", "src/app.tsx", true},
		{"src/*.{ts,tsx}", "src/app.js", false},
		{"a/[bc].go", "a/c.go", true},
		{"a/?.go", "a/xy.go", false},
	}
	for _, tc := range cases {
		t.Run(tc.glob+" "+tc.path, func(t *testing.T) {
			re, err := globRegexp(tc.glob)
			if err != nil {
				t.Fatal(err)
			}
			if got := re.MatchString(tc.path); got != tc.want {
				t.Errorf("match = %v, want %v (regexp %s)", got, tc.want, re)
			}
		})
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// skipDirs are never matched by a rule: VCS data, dependencies, build output
// and local scratch.
var skipDirs = map[string]bool{".git": true, "node_modules": true, ".ignored": true, "build": true}

func repoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// rulePaths reads the `paths:` list from a rule file's YAML frontmatter. It
// accepts the block form the rule files use:
//
//	paths:
//	  - "internal/mail/**"
func rulePaths(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || sc.Text() != "---" {
		return nil
	}
	var globs []string
	inPaths := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			break
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "paths:":
			inPaths = true
		case inPaths && strings.HasPrefix(trimmed, "- "):
			globs = append(globs, strings.Trim(strings.TrimSpace(trimmed[2:]), `"'`))
		default:
			inPaths = false
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return globs
}

// globRegexp translates a rule glob into an anchored regexp: `**` spans
// directories, `*` and `?` stay within one, `{a,b}` is a choice and `[...]`
// a character class.
func globRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	depth := 0
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		case c == '{':
			depth++
			b.WriteString("(?:")
		case c == '}' && depth > 0:
			depth--
			b.WriteString(")")
		case c == ',' && depth > 0:
			b.WriteString("|")
		case c == '[':
			end := strings.IndexByte(glob[i:], ']')
			if end < 0 {
				b.WriteString(regexp.QuoteMeta("["))
				continue
			}
			b.WriteString(glob[i : i+end+1])
			i += end
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
