package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/tools/seqtrace/browser"
	"github.com/Sourcehaven-BV/rela/tools/seqtrace/diagram"
)

type stitchFlags struct {
	in, out, static string
	titleRe         *regexp.Regexp
	opt             diagram.Options
}

func parseStitchFlags(args []string) (stitchFlags, error) {
	fs := flag.NewFlagSet("stitch", flag.ExitOnError)
	var f stitchFlags
	fs.StringVar(&f.in, "in", ".ignored/seqtrace-e2e/raw", "directory with server-*.jsonl and client-*.json")
	fs.StringVar(&f.out, "out", ".ignored/seqtrace-e2e/diagrams", "output directory")
	fs.StringVar(&f.static, "static", "internal/dataentry/static/v2", "built SPA, with source maps")
	collapse := fs.String("collapse", defaultCollapse, "regexp of packages drawn as part of their caller; empty to draw all")
	title := fs.String("title", `\.handle`, "regexp on Fn; the deepest matching call names the server handler")
	keep := fs.String("keep", "", "regexp of function names drawn even inside one package, such as access checks")
	fs.BoolVar(&f.opt.Values, "values", true, "label server arrows with argument and result summaries")
	_ = fs.Parse(args)

	var err error
	if *collapse != "" {
		if f.opt.Collapse, err = regexp.Compile(*collapse); err != nil {
			return f, err
		}
	}
	if *keep != "" {
		if f.opt.Keep, err = regexp.Compile(*keep); err != nil {
			return f, err
		}
	}
	f.titleRe, err = regexp.Compile(*title)
	return f, err
}

// stitch joins the browser recordings and server traces an e2e run wrote to
// one directory (RELA_E2E_SEQTRACE) into one diagram per SPA request.
func stitch(args []string) error {
	f, err := parseStitchFlags(args)
	if err != nil {
		return err
	}
	opt := f.opt
	roots, err := spanRoots(f.in)
	if err != nil {
		return err
	}
	clients, err := filepath.Glob(filepath.Join(f.in, "client-*.json"))
	if err != nil {
		return err
	}
	sort.Strings(clients)
	resolve := browser.StaticResolver(f.static)

	var pages []page
	unmatched, repeats := 0, 0
	// Scenario names must be unique: archplan keys sequences by them. A test
	// that sends the same request twice (a poll, a refetch) gets one page per
	// distinct call shape.
	shapes := map[string][]*diagram.Diagram{}
	for _, file := range clients {
		c, err := browser.ReadClient(file)
		if err != nil {
			return err
		}
		for _, rec := range c.Records {
			srv, ok := roots[rec.Span]
			if !ok {
				// The request never reached a handler, or its server's trace
				// is from another run.
				unmatched++
				continue
			}
			named := srv.root.Title(f.titleRe)
			opt.RootLabel = named.Fn
			frames := browser.Frames(rec.Stack, resolve)
			sc := browser.Stitch(c.Title, rec, frames, diagram.Build(srv.root, opt), named.Name())
			if srv.noACL {
				sc.Guards = "off"
			}
			seen := shapes[sc.Name]
			if slices.ContainsFunc(seen, func(d *diagram.Diagram) bool { return diagram.SameShape(d, sc.Diagram) }) {
				repeats++
				continue
			}
			shapes[sc.Name] = append(seen, sc.Diagram)
			if len(seen) > 0 {
				sc.Name = fmt.Sprintf("%s #%d", sc.Name, len(seen)+1)
			}
			p, err := stitchedPage(len(pages)+1, sc)
			if err != nil {
				return err
			}
			pages = append(pages, p)
		}
	}
	if err := os.MkdirAll(f.out, dirPerm); err != nil {
		return err
	}
	if err := writeOutputs(f.out, pages); err != nil {
		return err
	}
	index := filepath.Join(f.out, "index.html")
	if err := os.WriteFile(index, indexHTML(pages), filePerm); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr,
		"seqtrace: %d stitched diagrams (%d repeats skipped, %d requests without a server trace); open %s\n",
		len(pages), repeats, unmatched, index)
	return nil
}

// spanRoot is a server root call and what its server was run with.
type spanRoot struct {
	root  *diagram.Call
	noACL bool
}

// serverMeta is the sidecar the e2e fixture writes beside each server
// trace (e2e/tests/seqtrace.ts).
type serverMeta struct {
	ACL *bool `json:"acl"`
}

// spanRoots reads every server trace in dir and indexes its root calls by
// the span id on their request.
func spanRoots(dir string) (map[string]spanRoot, error) {
	files, err := filepath.Glob(filepath.Join(dir, "server-*.jsonl"))
	if err != nil {
		return nil, err
	}
	roots := map[string]spanRoot{}
	for _, f := range files {
		rs, err := readTrace(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		// Without a sidecar the ACL is unknown, and the trace keeps counting
		// as guard evidence.
		var meta serverMeta
		if b, err := os.ReadFile(strings.TrimSuffix(f, ".jsonl") + ".json"); err == nil {
			if err := json.Unmarshal(b, &meta); err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
		}
		noACL := meta.ACL != nil && !*meta.ACL
		for _, r := range rs {
			if span := browser.Span(r); span != "" {
				roots[span] = spanRoot{root: r, noACL: noACL}
			}
		}
	}
	return roots, nil
}

func stitchedPage(n int, sc diagram.Scenario) (page, error) {
	var mer, txt bytes.Buffer
	if err := sc.Diagram.Mermaid(&mer); err != nil {
		return page{}, err
	}
	if err := sc.Diagram.Text(&txt); err != nil {
		return page{}, err
	}
	return page{
		title: fmt.Sprintf("%03d %s", n, sc.Name), scenario: sc.Name, handler: sc.Handler,
		base: fmt.Sprintf("%03d-%s", n, fileSafe(sc.Base)), arrows: sc.Diagram.Arrows(),
		mermaid: mer.String(), text: txt.String(), diagram: sc.Diagram, guards: sc.Guards,
	}, nil
}
