// Package browser joins what the SPA did in the browser to what the server
// did for the same request. The browser recorder (recorder.js) keeps the
// JavaScript stack of every /api/ request and sends a span id with it; the
// traced server records that id on the request's root call. Stitch turns one
// such pair into a scenario whose call tree starts in the SPA module that
// made the request, crosses HTTP, and continues on the server.
package browser

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/tools/seqtrace/diagram"
)

// Client is one test's recording: its title and the requests its pages sent.
type Client struct {
	Title   string   `json:"title"`
	Records []Record `json:"records"`
}

// Record is one request the SPA sent.
type Record struct {
	Span   string `json:"span"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Stack  string `json:"stack"`
}

// ReadClient reads a client recording file.
func ReadClient(file string) (Client, error) {
	var c Client
	b, err := os.ReadFile(file)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: %w", file, err)
	}
	return c, nil
}

// Frame is one stack frame in the SPA's own code: the function and its
// source file relative to the frontend directory, such as
// src/components/entity/EntityDetail.vue.
type Frame struct {
	Fn     string
	Source string
}

// Resolver maps a script URL and a 1-based line and column to the source
// file they were built from.
type Resolver func(script string, line, col int) (string, bool)

// stackLine matches a V8 stack frame: "at fn (url:line:col)" or "at url:line:col".
var stackLine = regexp.MustCompile(`^\s*at (?:(.+?) \()?(\S+?):(\d+):(\d+)\)?$`)

// Frames returns the frames of stack that are in the SPA's own source,
// outermost first. Frames in libraries and in the recorder are left out.
func Frames(stack string, resolve Resolver) []Frame {
	var out []Frame
	for line := range strings.SplitSeq(stack, "\n") {
		m := stackLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[3])
		col, _ := strconv.Atoi(m[4])
		src, ok := resolve(m[2], ln, col)
		if !ok {
			continue
		}
		src, ok = ownSource(src)
		if !ok {
			continue
		}
		out = append(out, Frame{Fn: frameName(m[1], src), Source: src})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// ownSource returns a source path from "src/" on, or false for a library.
func ownSource(src string) (string, bool) {
	if strings.Contains(src, "node_modules/") {
		return "", false
	}
	i := strings.LastIndex("/"+src, "/src/")
	if i < 0 {
		return "", false
	}
	return src[i:], true
}

// frameName cleans a V8 function name; an anonymous frame is named after its file.
func frameName(fn, src string) string {
	fn = strings.TrimPrefix(strings.TrimPrefix(fn, "async "), "new ")
	fn = strings.TrimPrefix(fn, "Object.")
	if fn == "" || fn == "<anonymous>" {
		return path.Base(src)
	}
	return fn
}

// Participant is the diagram participant of an SPA source file: "web/" and
// its folder under src, such as web/components/entity, or web/src for a file
// directly in src. These are the component ids `archplan export-ts` gives
// the frontend under the prefix "web".
func Participant(src string) string {
	return "web/" + strings.TrimPrefix(path.Dir(src), "src/")
}

// Stitch builds the scenario of one request: Caller calls into the
// outermost SPA frame, each change of participant down the stack is a call,
// and the innermost SPA participant calls the server handler over HTTP with
// the server's own call tree below it.
func Stitch(title string, rec Record, frames []Frame, server *diagram.Diagram, handler string) diagram.Scenario {
	steps := []diagram.Step{}
	from := "Caller"
	parts := []string{}
	seen := map[string]bool{}
	addPart := func(p string) {
		if !seen[p] {
			seen[p] = true
			parts = append(parts, p)
		}
	}
	for _, f := range frames {
		to := Participant(f.Source)
		if to == from {
			continue
		}
		addPart(to)
		steps = append(steps, diagram.Step{From: from, To: to, Fn: f.Fn})
		from = to
	}
	hop := server.Root
	hop.From, hop.Via = from, "http"
	hop.Route = &diagram.Route{Method: rec.Method, Path: RouteShape(rec.Path)}
	hop.Span = rec.Span
	steps = append(steps, hop)
	for _, p := range server.Participants {
		addPart(p)
	}
	root := steps[len(steps)-1]
	for i := len(steps) - 2; i >= 0; i-- {
		s := steps[i]
		s.Children = []diagram.Step{root}
		root = s
	}
	return diagram.Scenario{
		Name:    title + ": " + rec.Method + " " + RouteShape(rec.Path),
		Handler: handler,
		Base:    rec.Span,
		Diagram: &diagram.Diagram{Participants: parts, Root: root},
	}
}

// apiVersion matches a version segment such as v1, which is not an id.
var apiVersion = regexp.MustCompile(`^v\d+$`)

// RouteShape replaces the path segments that look like ids (they hold a
// digit or an upper-case letter) with {id}, so a scenario name carries the
// route rather than the record.
func RouteShape(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if s == "" || strings.HasPrefix(s, "_") || apiVersion.MatchString(s) {
			continue
		}
		if strings.ContainsAny(s, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ%") {
			segs[i] = "{id}"
		}
	}
	return strings.Join(segs, "/")
}

// spanArg finds the span id seqtrace recorded on a request argument.
var spanArg = regexp.MustCompile(`\*http\.Request\{Span:([A-Za-z0-9-]+)\}`)

// Span returns the span id on the arguments of a server root call.
func Span(root *diagram.Call) string {
	for _, a := range root.Args {
		if m := spanArg.FindStringSubmatch(a); m != nil {
			return m[1]
		}
	}
	return ""
}

// StaticResolver resolves script URLs against the built SPA in dir: the
// script's source map when there is one, else a path that already names a
// source file (a Vite dev server serves /src/...).
func StaticResolver(dir string) Resolver {
	maps := map[string]*SourceMap{}
	return func(script string, line, col int) (string, bool) {
		u, err := url.Parse(script)
		if err != nil {
			return "", false
		}
		p := path.Clean("/" + u.Path)
		if strings.HasPrefix(p, "/src/") {
			return p[1:], true
		}
		m, ok := maps[p]
		if !ok {
			m, _ = LoadSourceMap(dir + p + ".map")
			maps[p] = m
		}
		if m == nil {
			return "", false
		}
		return m.Source(line, col)
	}
}
