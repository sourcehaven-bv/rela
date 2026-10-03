// Command seqtrace builds call-traced binaries and draws sequence diagrams
// from their traces. See tools/seqtrace/README.md.
//
//	seqtrace overlay -out DIR [-tags T] [-exclude RE] PATTERN...
//	seqtrace diagram -in TRACE -out DIR [-root RE] [-collapse RE] [-depth N] [-min N] [-values=false]
//	seqtrace diff [-values] BASE_DIR HEAD_DIR
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/tools/seqtrace/diagram"
	"github.com/Sourcehaven-BV/rela/tools/seqtrace/rewrite"
)

const (
	filePerm = 0o600
	dirPerm  = 0o750
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "overlay":
		err = overlay(os.Args[2:])
	case "diagram":
		err = diagrams(os.Args[2:])
	case "diff":
		err = diffDirs(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "seqtrace:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: seqtrace overlay|diagram|diff [flags] ...")
	os.Exit(2)
}

type listedPkg struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Module     *struct{ Path string }
}

func overlay(args []string) error {
	fs := flag.NewFlagSet("overlay", flag.ExitOnError)
	out := fs.String("out", ".ignored/seqtrace", "output directory for rewritten sources and overlay.json")
	tags := fs.String("tags", "", "build tags, as for go build")
	exclude := fs.String("exclude", "", "regexp of import paths to leave uninstrumented")
	_ = fs.Parse(args)
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./internal/...", "./cmd/..."}
	}
	var excl *regexp.Regexp
	if *exclude != "" {
		re, err := regexp.Compile(*exclude)
		if err != nil {
			return err
		}
		excl = re
	}

	pkgs, err := goList(*tags, patterns)
	if err != nil {
		return err
	}
	absOut, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	srcRoot := filepath.Join(absOut, "src")
	if rmErr := os.RemoveAll(srcRoot); rmErr != nil {
		return rmErr
	}

	replace := map[string]string{}
	var nPkgs int
	for _, p := range pkgs {
		if p.Module == nil || strings.HasPrefix(p.ImportPath, rewrite.Runtime) ||
			(excl != nil && excl.MatchString(p.ImportPath)) {

			continue
		}
		label := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, p.Module.Path), "/")
		n, rerr := rewritePkg(p, label, filepath.Join(srcRoot, label), replace)
		if rerr != nil {
			return rerr
		}
		if n > 0 {
			nPkgs++
		}
	}
	b, err := json.MarshalIndent(map[string]any{"Replace": replace}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(absOut, "overlay.json")
	if err := os.WriteFile(path, b, filePerm); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "seqtrace: instrumented %d files in %d packages; overlay at %s\n", len(replace), nPkgs, path)
	return nil
}

func rewritePkg(p listedPkg, label, dst string, replace map[string]string) (int, error) {
	n := 0
	for _, name := range p.GoFiles {
		orig := filepath.Join(p.Dir, name)
		src, err := os.ReadFile(orig)
		if err != nil {
			return n, err
		}
		out, changed, err := rewrite.File(orig, src, label)
		if err != nil {
			return n, fmt.Errorf("%s: %w", orig, err)
		}
		if !changed {
			continue
		}
		if err := os.MkdirAll(dst, dirPerm); err != nil {
			return n, err
		}
		target := filepath.Join(dst, name)
		// target is under -out, built from go list paths.
		if err := os.WriteFile(target, out, filePerm); err != nil { //nolint:gosec // see above
			return n, err
		}
		replace[orig] = target
		n++
	}
	return n, nil
}

func goList(tags string, patterns []string) ([]listedPkg, error) {
	args := []string{"list", "-json=ImportPath,Dir,GoFiles,Module"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, patterns...)
	cmd := exec.CommandContext(context.Background(), "go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w\n%s", err, stderr.String())
	}
	var pkgs []listedPkg
	dec := json.NewDecoder(bytes.NewReader(stdout))
	for {
		var p listedPkg
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

type diagramFlags struct {
	in, out, names string
	minArrows      int
	rootRe         *regexp.Regexp
	titleRe        *regexp.Regexp
	opt            diagram.Options
}

func parseDiagramFlags(args []string) (diagramFlags, error) {
	fs := flag.NewFlagSet("diagram", flag.ExitOnError)
	var f diagramFlags
	fs.StringVar(&f.in, "in", ".ignored/seqtrace/trace.jsonl", "trace file written by an instrumented binary")
	fs.StringVar(&f.out, "out", ".ignored/seqtrace/diagrams", "output directory")
	fs.StringVar(&f.names, "names", "",
		"file with one scenario name per root, in order; a line \"-\" skips that root")
	root := fs.String("root", "", "regexp on pkg.Func selecting which root calls to draw")
	collapse := fs.String("collapse", defaultCollapse, "regexp of packages drawn as part of their caller; empty to draw all")
	fs.IntVar(&f.opt.MaxDepth, "depth", 0, "maximum arrow nesting; 0 for no limit")
	fs.IntVar(&f.minArrows, "min", 2, "skip diagrams with fewer arrows than this")
	title := fs.String("title", `\.handle`, "regexp on Fn; the deepest matching call names each diagram")
	fs.BoolVar(&f.opt.Values, "values", true, "label arrows with argument and result summaries")
	_ = fs.Parse(args)

	var err error
	if f.titleRe, err = regexp.Compile(*title); err != nil {
		return f, err
	}
	if *root != "" {
		if f.rootRe, err = regexp.Compile(*root); err != nil {
			return f, err
		}
	}
	if *collapse != "" {
		if f.opt.Collapse, err = regexp.Compile(*collapse); err != nil {
			return f, err
		}
	}
	return f, nil
}

// page is one rendered diagram, before its HTML is written.
type page struct {
	title, scenario, handler string
	base                     string // file name without extension
	arrows                   int
	mermaid, text            string
	diagram                  *diagram.Diagram
}

// stepsFile holds every scenario's step tree, for seqtrace diff.
const stepsFile = "steps.json"

func writeSteps(dir string, pages []page) error {
	ss := make([]diagram.Scenario, len(pages))
	for i, p := range pages {
		ss[i] = diagram.Scenario{Name: p.scenario, Handler: p.handler, Base: p.base, Diagram: p.diagram}
	}
	b, err := json.Marshal(ss)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stepsFile), b, filePerm)
}

func readSteps(dir string) ([]diagram.Scenario, error) {
	b, err := os.ReadFile(filepath.Join(dir, stepsFile))
	if err != nil {
		return nil, fmt.Errorf("%s: %w (was it written by seqtrace diagram?)", dir, err)
	}
	var ss []diagram.Scenario
	if err := json.Unmarshal(b, &ss); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(dir, stepsFile), err)
	}
	return ss, nil
}

// diffDirs compares two diagram directories and prints the report.
func diffDirs(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	values := fs.Bool("values", false, "compare argument and result values too, not only call shapes")
	_ = fs.Parse(args)
	if fs.NArg() != 2 {
		return errors.New("usage: seqtrace diff [-values] BASE_DIR HEAD_DIR")
	}
	base, err := readSteps(fs.Arg(0))
	if err != nil {
		return err
	}
	head, err := readSteps(fs.Arg(1))
	if err != nil {
		return err
	}
	_, err = diagram.Compare(os.Stdout, base, head, *values)
	return err
}

func diagrams(args []string) error {
	f, err := parseDiagramFlags(args)
	if err != nil {
		return err
	}
	names, err := readNames(f.names)
	if err != nil {
		return err
	}
	roots, err := readTrace(f.in)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(f.out, dirPerm); err != nil {
		return err
	}

	var pages []page
	i := -1
	for _, r := range roots {
		if f.rootRe != nil && !f.rootRe.MatchString(r.Name()) {
			continue
		}
		i++
		scenario := ""
		if i < len(names) {
			scenario = names[i]
		}
		if scenario == "-" {
			continue
		}
		named := r.Title(f.titleRe)
		f.opt.RootLabel = named.Fn
		d := diagram.Build(r, f.opt)
		arrows := d.Arrows()
		if arrows < f.minArrows {
			continue
		}
		var mer, txt bytes.Buffer
		if err := d.Mermaid(&mer); err != nil {
			return err
		}
		if err := d.Text(&txt); err != nil {
			return err
		}
		n := len(pages) + 1
		p := page{scenario: scenario, handler: named.Name(), arrows: arrows, mermaid: mer.String(), text: txt.String(),
			diagram: d,
			title:   fmt.Sprintf("%03d %s", n, named.Name()), base: fmt.Sprintf("%03d-%s", n, fileSafe(named.Fn))}
		if scenario != "" {
			p.title = fmt.Sprintf("%03d %s", n, scenario)
			p.base = fmt.Sprintf("%03d-%s", n, fileSafe(scenario))
		}
		pages = append(pages, p)
	}
	if err := writeOutputs(f.out, pages); err != nil {
		return err
	}
	if f.names != "" && i+1 != len(names) {
		// Names are assigned by position, so a count mismatch means the labels
		// are probably shifted.
		fmt.Fprintf(os.Stderr, "seqtrace: warning: %d names for %d roots; scenario labels may be wrong\n", len(names), i+1)
	}
	index := filepath.Join(f.out, "index.html")
	if err := os.WriteFile(index, indexHTML(pages), filePerm); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "seqtrace: %d diagrams from %d roots; open %s\n", len(pages), len(roots), index)
	return nil
}

// writeOutputs writes each page as HTML, Markdown and text, and the step
// trees of all of them.
func writeOutputs(dir string, pages []page) error {
	for i, p := range pages {
		if err := writePage(dir, pages, i); err != nil {
			return err
		}
		md := fmt.Sprintf("# %s\n\n`%s`\n\n```mermaid\n%s```\n", p.title, p.handler, p.mermaid)
		if err := os.WriteFile(filepath.Join(dir, p.base+".md"), []byte(md), filePerm); err != nil {
			return err
		}
		txt := fmt.Sprintf("# %s\n# %s\n%s", p.title, p.handler, p.text)
		if err := os.WriteFile(filepath.Join(dir, p.base+".txt"), []byte(txt), filePerm); err != nil {
			return err
		}
	}
	return writeSteps(dir, pages)
}

// writePage writes pages[i] as HTML with links to its neighbors.
func writePage(dir string, pages []page, i int) error {
	p := pages[i]
	e := html.EscapeString
	prev, next := `<span></span>`, `<span></span>`
	if i > 0 {
		prev = fmt.Sprintf(`<a href="%s.html">&larr; %s</a>`, pages[i-1].base, e(pages[i-1].title))
	}
	if i+1 < len(pages) {
		next = fmt.Sprintf(`<a href="%s.html">%s &rarr;</a>`, pages[i+1].base, e(pages[i+1].title))
	}
	body := fmt.Sprintf(`<nav>%s <a href="index.html">index</a> %s</nav>
<h1>%s</h1>
<p><code>%s</code> &middot; %d arrows &middot; <a href="%s.md">markdown</a> &middot; <a href="%s.txt">text</a></p>
<pre class="mermaid">
%s</pre>
`, prev, next, e(p.title), e(p.handler), p.arrows, p.base, p.base, e(p.mermaid))
	return os.WriteFile(filepath.Join(dir, p.base+".html"), fmt.Appendf(nil, pageTmpl, e(p.title), mermaidJS, body), filePerm)
}

func indexHTML(pages []page) []byte {
	e := html.EscapeString
	var rows strings.Builder
	for i, p := range pages {
		name := p.scenario
		if name == "" {
			name = p.handler
		}
		fmt.Fprintf(&rows, "<tr><td>%d</td><td><a href=\"%s.html\">%s</a></td><td><code>%s</code></td><td>%d</td></tr>\n",
			i+1, p.base, e(name), e(p.handler), p.arrows)
	}
	body := fmt.Sprintf(`<h1>seqtrace scenarios</h1>
<table>
<tr><th>#</th><th>Scenario</th><th>Handler</th><th>Arrows</th></tr>
%s</table>
`, rows.String())
	return fmt.Appendf(nil, pageTmpl, "seqtrace", "", body)
}

func readTrace(path string) ([]*diagram.Call, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return diagram.Read(f)
}

// readNames returns the non-empty lines of path, or nil when path is empty.
func readNames(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for l := range strings.SplitSeq(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			names = append(names, l)
		}
	}
	return names, nil
}

// defaultCollapse names value and helper packages. Their calls add many
// arrows to every diagram without showing a handoff between components.
const defaultCollapse = `^internal/(entity|metamodel|principal|frontmatter|storage|store/storeutil|markdown|cache|computed)$`

func fileSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '_'
	}, s)
}

// pageTmpl takes the title, an optional script and the body.
const pageTmpl = `<!doctype html>
<meta charset="utf-8">
<title>%s</title>
<style>
body{font-family:system-ui;margin:2em}
h1{font-size:1.3em} nav{display:flex;justify-content:space-between;gap:1em}
table{border-collapse:collapse} td,th{padding:.3em .8em;text-align:left;border-bottom:1px solid #ddd}
</style>
%s
%s`

// mermaidJS loads one pinned, single-file Mermaid build with an integrity
// hash: the pages embed trace values. Update the version and hash together.
const mermaidJS = `<script src="https://cdn.jsdelivr.net/npm/mermaid@11.17.2/dist/mermaid.min.js"
  integrity="sha384-EOXBFmc3gx5mb+vn0vPvvGqACToJD24hhacX5Yx+8NUUQrHIle/Qi5Bg9o3zKwW2"
  crossorigin="anonymous"></script>
<script>
mermaid.initialize({startOnLoad: true, maxTextSize: 10000000, maxEdges: 100000,
  sequence: {showSequenceNumbers: true, mirrorActors: false, wrap: true, width: 220}});
</script>`
