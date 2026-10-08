package agentrules

import (
	"bufio"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const rulesDir = ".claude/rules"

// TestRulePathsMatchFiles fails when a rule file has no paths, or when one of
// its globs matches no file git knows about other than tests. Test files do
// not count: a glob kept alive by a leftover test would hide a moved package.
func TestRulePathsMatchFiles(t *testing.T) {
	root := repoRoot(t)
	files := repoFiles(t, root)
	rules := ruleFiles(t, root)
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
				nonTest := func(f string) bool { return re.MatchString(f) && !strings.HasSuffix(f, "_test.go") }
				if !slices.ContainsFunc(files, nonTest) {
					t.Errorf("%q matches no file outside tests; update it after a move or rename", g)
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
		{"a/[!b].go", "a/b.go", false},
		{"a/[!b].go", "a/c.go", true},
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

// repoFiles lists the files git tracks plus untracked ones it does not
// ignore, so the result is the same on a developer machine and in CI: local
// leftovers and generated files do not count.
func repoFiles(t *testing.T, root string) []string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var files []string
	for f := range strings.SplitSeq(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

// ruleFiles lists the rule files, including those in subdirectories, which
// Claude Code also loads.
func ruleFiles(t *testing.T, root string) []string {
	t.Helper()
	var rules []string
	err := filepath.WalkDir(filepath.Join(root, rulesDir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			rules = append(rules, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return rules
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
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			// Blank and comment lines do not end the list.
		case inPaths && strings.HasPrefix(trimmed, "- "):
			globs = append(globs, listItem(trimmed[2:]))
		case strings.HasPrefix(trimmed, "paths:"):
			t.Fatalf("%s: write paths as a block list, one \"- glob\" per line", path)
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
// a character class (`[!...]` negated). Unlike Claude Code's matcher, `*`
// also matches a leading dot; the rule globs name no dotfiles.
// listItem returns a YAML list item's value without quotes or a trailing
// comment.
func listItem(s string) string {
	s = strings.TrimSpace(s)
	if q := s[:min(1, len(s))]; q == `"` || q == "'" {
		if end := strings.Index(s[1:], q); end >= 0 {
			return s[1 : end+1]
		}
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

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
			class := glob[i : i+end+1]
			if strings.HasPrefix(class, "[!") {
				class = "[^" + class[2:] // glob negation
			}
			b.WriteString(class)
			i += end
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func TestListItem(t *testing.T) {
	for in, want := range map[string]string{
		`"internal/mail/**"`:       "internal/mail/**",
		`'a/*.go' # note`:          "a/*.go",
		`a/**  # trailing comment`: "a/**",
		`"a/#b"`:                   "a/#b",
	} {
		if got := listItem(in); got != want {
			t.Errorf("listItem(%q) = %q, want %q", in, got, want)
		}
	}
}
