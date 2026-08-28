package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// APIClient issues one HTTP request against the documented project's API.
//
// Like [Capturer] this is a consumer-side interface so the core docs package
// never imports the data-entry app or a server; the CLI injects the concrete
// implementation. A nil APIClient makes api{} fail loud rather than silently
// skip — a skipped assertion is indistinguishable from a passing one, which is
// the failure mode this whole feature exists to remove.
//
// Unlike Capturer this needs no browser and no built frontend, so api{} can
// gate CI unconditionally.
type APIClient interface {
	// Do issues the request and returns the response. A transport-level
	// failure is an error; an HTTP error STATUS is a normal response, since
	// asserting a 403 is the point of the verb.
	Do(ctx context.Context, req APIRequest) (APIResponse, error)
	// Close tears down any temp project / server stood up to serve requests.
	Close() error
}

// APIRequest is a plain DTO so the interface carries no runtime internals.
type APIRequest struct {
	// ProjectDir is the documented project; Seed is replayed into a temp copy
	// so the API serves the same entities the manual created.
	ProjectDir string
	Seed       []SeedOp

	Method string // default GET
	Path   string // e.g. "/api/v1/entities/policy/POL-1"
	// As is the role to act as, mapped to a principal assigned that role in
	// acl.yaml. Empty ⇒ the harness picks a role that can read.
	As string
	// Body is an optional JSON request body.
	Body string
}

// APIResponse is the part of the response an assertion can claim about.
type APIResponse struct {
	Status int
	Body   string
}

// api{} asserts an API contract: status, machine-readable error code, or that
// two requests answer IDENTICALLY.
//
// # Why `error` means the code, not the prose
//
// A message is prose and will be reworded; the code is the contract. Asserting
// the prose produces a test that fails on a copy edit and passes on a genuine
// behavior change — precisely backwards.
//
// # Why `identical_to` exists
//
// The existence-oracle property — "a denied read is indistinguishable from a
// real 404" — is not a claim about one response. It is a claim about two
// responses being the SAME, and no single-response assertion can express it.
// It is currently pinned only in Go (viewworld_absent_test.go), where the
// security property is invisible to anyone reading the manual that promises it.
func (dr *docRuntime) luaAPI(ls *lua.LState) int {
	tbl := argTable(ls)
	if tbl == nil {
		return dr.luaFail(ls, `api: expects a table, e.g. api{path="/api/v1/entities/policy/POL-1", status=200}`)
	}

	if rejectUnknownKeys(dr, ls, "api", tbl,
		"path", "method", "as", "body", "status", "error", "identical_to") {
		return 0
	}

	path := fieldString(ls, tbl, "path")
	if path == "" {
		return dr.luaFail(ls, "api: `path` is required")
	}

	wantStatus := fieldInt(ls, tbl, "status", 0)
	wantError := fieldString(ls, tbl, "error")
	identicalTo := fieldTable(tbl, "identical_to")

	if wantStatus == 0 && wantError == "" && identicalTo == nil {
		return dr.luaFail(ls, "api{path=%q}: asserts nothing. Give at least one of "+
			"status=, error= or identical_to= — a call with no claim passes whatever "+
			"the server does", path)
	}
	if dr.apiClient == nil {
		return dr.luaFail(ls, "api{path=%q}: no API client available: %s", path, dr.apiClientErr)
	}

	req := APIRequest{
		ProjectDir: dr.projectDir,
		Seed:       dr.seedOps,
		Method:     fieldString(ls, tbl, "method"),
		Path:       path,
		As:         fieldString(ls, tbl, "as"),
		Body:       fieldString(ls, tbl, "body"),
	}
	resp, err := dr.apiClient.Do(dr.ctx, req)
	if err != nil {
		return dr.luaFail(ls, "api{path=%q}: %v", path, err)
	}

	if msg := checkAPI(path, resp, wantStatus, wantError); msg != "" {
		return dr.luaFail(ls, "%s", msg)
	}

	if identicalTo != nil {
		other := APIRequest{
			ProjectDir: dr.projectDir,
			Seed:       dr.seedOps,
			Method:     fieldStringOf(identicalTo, "method"),
			Path:       fieldStringOf(identicalTo, "path"),
			As:         fieldStringOf(identicalTo, "as"),
			Body:       fieldStringOf(identicalTo, "body"),
		}
		if rejectUnknownKeys(dr, ls, "api identical_to", identicalTo, "path", "method", "as", "body") {
			return 0
		}
		if other.Path == "" {
			return dr.luaFail(ls, "api{path=%q}: identical_to needs its own `path`", path)
		}
		otherResp, oerr := dr.apiClient.Do(dr.ctx, other)
		if oerr != nil {
			return dr.luaFail(ls, "api{identical_to=%q}: %v", other.Path, oerr)
		}
		if msg := checkIdentical(path, other.Path, resp, otherResp); msg != "" {
			return dr.luaFail(ls, "%s", msg)
		}
	}
	return 0
}

// checkAPI compares one response against the status/error claims.
func checkAPI(path string, resp APIResponse, wantStatus int, wantError string) string {
	var b strings.Builder
	if wantStatus != 0 && resp.Status != wantStatus {
		fmt.Fprintf(&b, "api{path=%q} failed\n  claimed status: %d\n  actual status:  %d",
			path, wantStatus, resp.Status)
		if body := strings.TrimSpace(resp.Body); body != "" {
			fmt.Fprintf(&b, "\n  body: %s", truncate(body, bodyExcerpt))
		}
		return b.String()
	}
	if wantError != "" {
		got := problemCode(resp.Body)
		if got != wantError {
			fmt.Fprintf(&b, "api{path=%q} failed\n  claimed error: %s\n  actual error:  %s",
				path, wantError, orNone(got))
			fmt.Fprintf(&b, "\n  body: %s", truncate(strings.TrimSpace(resp.Body), bodyExcerpt))
			return b.String()
		}
	}
	return ""
}

// checkIdentical is the existence-oracle assertion: two responses must be
// indistinguishable in status and body. A "not permitted" where the other says
// "not found" is a channel telling an unauthorized caller that something exists.
//
// # Why `instance` is excluded
//
// RFC 7807 problem details carry `instance` = the requested URL, so two
// responses to DIFFERENT urls necessarily differ there and a raw byte compare
// can never pass — it would report a leak on every pair, which is a check that
// fails always rather than one that checks anything. `instance` reflects only
// what the caller already typed, so it discloses nothing. Everything else —
// status, type, title, and any other field — must match exactly.
func checkIdentical(pathA, pathB string, a, b APIResponse) string {
	if a.Status == b.Status && normalizeProblem(a.Body) == normalizeProblem(b.Body) {
		return ""
	}
	var s strings.Builder
	fmt.Fprintf(&s, "api{identical_to} failed — the two responses differ, so the pair "+
		"distinguishes a denied read from a missing one\n")
	fmt.Fprintf(&s, "  %s → %d %s\n", pathA, a.Status, truncate(strings.TrimSpace(a.Body), pairExcerpt))
	fmt.Fprintf(&s, "  %s → %d %s", pathB, b.Status, truncate(strings.TrimSpace(b.Body), pairExcerpt))
	return s.String()
}

// problemCode pulls the machine-readable code out of a problem-details body.
// Returns "" when the body is not JSON or carries no code — the caller renders
// that as "(none)" rather than pretending the claim matched.
func problemCode(body string) string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return ""
	}
	for _, key := range []string{"code", "type", "error"} {
		if v, ok := doc[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// normalizeProblem drops the `instance` member so two responses to different
// urls can be compared for everything else. A body that is not JSON is returned
// unchanged — an unparseable body is compared verbatim rather than silently
// treated as equal to another unparseable one.
func normalizeProblem(body string) string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return body
	}
	delete(doc, "instance")
	out, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	return string(out)
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// bodyExcerpt caps how much of a response body a failure message prints:
// enough to see the problem code and title, not so much that the real claim
// scrolls off the screen.
const bodyExcerpt = 400

// pairExcerpt is shorter because identical_to prints TWO bodies side by side.
const pairExcerpt = 300

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}

// fieldTable returns a nested table field, or nil when absent.
func fieldTable(tbl *lua.LTable, key string) *lua.LTable {
	if t, ok := tbl.RawGetString(key).(*lua.LTable); ok {
		return t
	}
	return nil
}

// fieldStringOf reads a string field without a runtime handy (nested tables).
func fieldStringOf(tbl *lua.LTable, key string) string {
	if s, ok := tbl.RawGetString(key).(lua.LString); ok {
		return string(s)
	}
	return ""
}
