package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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
	// Header carries response headers, because the body is not the only
	// existence-oracle channel: a denied GET that emits an ETag (or honors
	// If-None-Match with a 304) confirms the entity exists while the body says
	// nothing. Pinned in Go by TestACLGet_ETagSuppressedOnDeny; identical_to
	// compares it so a manual can make the same claim.
	Header map[string][]string
}

// # Islands share ONE project, and run in order
//
// Every island of a document — api{}, screenshot{}, page{} — reads and writes
// the SAME temp project, and they execute top-to-bottom in source order. So a
// write made here IS visible to everything below it: an api{} POST that
// publishes a copy can be photographed by a later screenshot{}, which is the
// figure this sharing was built for.
//
// What is NOT guaranteed:
//
//   - Nothing is visible to an island ABOVE the write. Order is source order,
//     with no lookahead.
//   - Writes are not isolated or rolled back between islands. There is one
//     store for the document and every write persists into the next island, so
//     an api{} that mutates state changes what later assertions see. That is
//     the point, but it means an island is not a fixture boundary.
//   - Nothing is shared BETWEEN documents. Two manuals built in one run get
//     separate projects, so a write in one is invisible to the other.
//   - Asynchronous work is not awaited. A version row written by the postgres
//     sweep arrives on a debounce, so a figure depending on one must say so
//     (screenshot{await_versions=N}) rather than assume the next island sees it.
//
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
func (b *tierBBindings) luaAPI(ls *lua.LState) int {
	tbl := argTable(ls)
	if tbl == nil {
		return b.luaFail(ls, `api: expects a table, e.g. api{path="/api/v1/entities/policy/POL-1", status=200}`)
	}

	if rejectUnknownKeys(b, ls, "api", tbl,
		"path", "method", "as", "body", "status", "error", "identical_to", "has", "absent", "emit") {

		return 0
	}
	show := fieldBoolDefault(ls, tbl, "emit", true)

	path := fieldString(ls, tbl, "path")
	if path == "" {
		return b.luaFail(ls, "api: `path` is required")
	}

	wantStatus := fieldInt(ls, tbl, "status", 0)
	wantError := fieldString(ls, tbl, "error")
	identicalTo := fieldTable(tbl, "identical_to")

	has, herr := claimList(tbl, "has")
	if herr != nil {
		return b.luaFail(ls, "api{path=%q}: %v", path, herr)
	}
	absent, aerr := claimList(tbl, "absent")
	if aerr != nil {
		return b.luaFail(ls, "api{path=%q}: %v", path, aerr)
	}

	if wantStatus == 0 && wantError == "" && identicalTo == nil && len(has) == 0 && len(absent) == 0 {
		return b.luaFail(ls, "api{path=%q}: asserts nothing. Give at least one of "+
			"status=, error=, has=, absent= or identical_to= — a call with no claim passes "+
			"whatever the server does", path)
	}
	// An `absent=` claim against a response that FAILED is vacuous: an error
	// body contains none of the strings, so every absent= would pass while
	// proving nothing about what the endpoint discloses. Requiring the status
	// claim alongside it means the body claim is only ever read on a response
	// the manual has already pinned.
	if len(absent) > 0 && wantStatus == 0 && wantError == "" {
		return b.luaFail(ls, "api{path=%q}: absent= needs a status= claim beside it. An error "+
			"response contains none of the strings either, so absent= alone would pass on a "+
			"500 and prove nothing about what a successful response withholds", path)
	}
	if b.apiClient == nil {
		return b.luaFail(ls, "api{path=%q}: no API client available: %s", path, b.apiClientErr)
	}

	req := APIRequest{
		ProjectDir: b.projectDir,
		Seed:       b.seed(),
		Method:     fieldString(ls, tbl, "method"),
		Path:       path,
		As:         fieldString(ls, tbl, "as"),
		Body:       fieldString(ls, tbl, "body"),
	}
	resp, err := b.apiClient.Do(b.ctx, req)
	if err != nil {
		return b.luaFail(ls, "api{path=%q}: %v", path, err)
	}

	if msg := checkAPI(path, resp, wantStatus, wantError, has, absent); msg != "" {
		return b.luaFail(ls, "%s", msg)
	}

	if identicalTo != nil {
		return b.assertIdentical(ls, req, resp, identicalTo, show)
	}

	if show {
		b.emit(apiEvidence(req, resp, wantError, has, absent).render())
	}
	return 0
}

// apiEvidence states one request and what the server answered.
//
// The principal is named in the sentence because an API status is meaningless
// without it: 404 for a reader and 200 for an editor is the FEATURE, and a
// figure showing only "404" invites the reader to conclude the entity is gone.
func apiEvidence(req APIRequest, resp APIResponse, wantError string, has, absent []string) evidence {
	as := req.As
	if as == "" {
		as = "a reader who may read"
	} else {
		as = fmt.Sprintf("`%s`", as)
	}
	method := req.Method
	if method == "" {
		method = "GET"
	}
	ev := evidence{
		claim: fmt.Sprintf("%s `%s` as %s answers **%d %s**.",
			method, req.Path, as, resp.Status, statusPhrase(resp.Status)),
	}
	var notes []string
	if wantError != "" {
		notes = append(notes, fmt.Sprintf("Error code `%s`.", wantError))
	}
	// The body claims are the interesting half when they are present: "404"
	// says the row was withheld, while absent= says the response did not name
	// a thing the caller may not see. Spell both out rather than leaving the
	// reader to infer them from a status.
	if len(has) > 0 {
		notes = append(notes, fmt.Sprintf("The response names %s.", quotedList(has)))
	}
	if len(absent) > 0 {
		notes = append(notes, fmt.Sprintf("It does not name %s.", quotedList(absent)))
	}
	ev.note = strings.Join(notes, " ")
	return ev
}

// quotedList renders a claim list for prose, so evidence reads as a sentence
// rather than as a Go slice dump.
func quotedList(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, s := range items {
		quoted = append(quoted, fmt.Sprintf("`%s`", s))
	}
	return strings.Join(quoted, ", ")
}

// identicalEvidence states the indistinguishability claim.
//
// This one carries the security property a reader most needs spelled out: two
// requests answering the same way is what makes a denied read unable to confirm
// that an entity exists. Rendering only the two statuses would let a reader
// conclude "both 404" without seeing that the SAMENESS is the point.
func identicalEvidence(req, other APIRequest, resp APIResponse) evidence {
	return evidence{
		claim: fmt.Sprintf("`%s` and `%s` answer **identically** (%d %s).",
			req.Path, other.Path, resp.Status, statusPhrase(resp.Status)),
		note: "Byte-for-byte the same, headers included — so the answer cannot be used to " +
			"tell whether the entity exists.",
	}
}

// statusPhrase names an HTTP status for a reader who does not know the codes.
//
// http.StatusText covers every registered code, so there is no local table to
// drift from the standard library's.
func statusPhrase(code int) string {
	if text := http.StatusText(code); text != "" {
		return text
	}
	return "Unknown"
}

// assertIdentical runs the two-request indistinguishability claim.
//
// Split out of luaAPI because the claim is a self-contained second request with
// its own validation, and inlining it made the caller's nesting hard to follow.
func (b *tierBBindings) assertIdentical(
	ls *lua.LState, req APIRequest, resp APIResponse,
	identicalTo *lua.LTable, show bool,
) int {
	other := APIRequest{
		ProjectDir: b.projectDir,
		Seed:       b.seed(),
		Method:     fieldStringOf(identicalTo, "method"),
		Path:       fieldStringOf(identicalTo, "path"),
		As:         fieldStringOf(identicalTo, "as"),
		Body:       fieldStringOf(identicalTo, "body"),
	}
	if rejectUnknownKeys(b, ls, "api identical_to", identicalTo, "path", "method", "as", "body") {
		return 0
	}
	if other.Path == "" {
		return b.luaFail(ls, "api{path=%q}: identical_to needs its own `path`", req.Path)
	}
	// Comparing a request with itself is trivially true — a claimless call
	// wearing a claim's clothes, which is the class this feature refuses.
	if other.Path == req.Path && other.As == req.As && other.Method == req.Method && other.Body == req.Body {
		return b.luaFail(ls, "api{path=%q}: identical_to names the SAME request, which is "+
			"always true. The claim is that two DIFFERENT requests are indistinguishable — "+
			"vary the path or the principal", req.Path)
	}
	otherResp, oerr := b.apiClient.Do(b.ctx, other)
	if oerr != nil {
		return b.luaFail(ls, "api{identical_to=%q}: %v", other.Path, oerr)
	}
	if msg := checkIdentical(req.Path, other.Path, resp, otherResp); msg != "" {
		return b.luaFail(ls, "%s", msg)
	}
	if show {
		b.emit(identicalEvidence(req, other, resp).render())
	}
	return 0
}

// checkAPI compares one response against the status/error claims.
func checkAPI(path string, resp APIResponse, wantStatus int, wantError string, has, absent []string) string {
	// Both claims are reported when both fail: a wrong status used to mask a
	// wrong error code, costing an extra fix-and-rerun cycle on a red build.
	var problems []string
	if wantStatus != 0 && resp.Status != wantStatus {
		problems = append(problems, fmt.Sprintf("  claimed status: %d\n  actual status:  %d",
			wantStatus, resp.Status))
	}
	if wantError != "" {
		if got := problemCode(resp.Body); got != wantError {
			problems = append(problems, fmt.Sprintf("  claimed error: %s\n  actual error:  %s",
				wantError, orNone(got)))
		}
	}
	for _, want := range has {
		if !strings.Contains(resp.Body, want) {
			problems = append(problems, fmt.Sprintf("  body must contain: %q\n  but it does not", want))
		}
	}
	for _, unwanted := range absent {
		if strings.Contains(resp.Body, unwanted) {
			problems = append(problems, fmt.Sprintf("  body must NOT contain: %q\n  but it does — this is a "+
				"disclosure, not a mismatch", unwanted))
		}
	}
	if len(problems) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "api{path=%q} failed\n%s", path, strings.Join(problems, "\n"))
	if body := strings.TrimSpace(resp.Body); body != "" {
		fmt.Fprintf(&b, "\n  body: %s", truncate(body, bodyExcerpt))
	}
	return b.String()
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
	hdrA, hdrB := oracleHeaders(a.Header), oracleHeaders(b.Header)
	if a.Status == b.Status &&
		normalizeProblem(a.Body) == normalizeProblem(b.Body) &&
		hdrA == hdrB {

		return ""
	}
	var s strings.Builder
	fmt.Fprintf(&s, "api{identical_to} failed — the two responses differ, so the pair "+
		"distinguishes a denied read from a missing one\n")
	fmt.Fprintf(&s, "  %s → %d %s\n", pathA, a.Status, truncate(strings.TrimSpace(a.Body), pairExcerpt))
	fmt.Fprintf(&s, "  %s → %d %s", pathB, b.Status, truncate(strings.TrimSpace(b.Body), pairExcerpt))
	if hdrA != hdrB {
		fmt.Fprintf(&s, "\n  differing headers: %s vs %s", orNone(hdrA), orNone(hdrB))
	}
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

// oracleHeaders renders the response headers that can BY THEMSELVES disclose
// existence, as a stable comparable string.
//
// Only a deliberate allowlist is compared, not every header: Date and
// Content-Length vary for reasons that disclose nothing, so comparing
// everything would fail always — the same trap the `instance` member sets.
// ETag is the documented channel (a denied GET must not emit one, or a replayed
// If-None-Match turns into a 304 that confirms the entity exists).
func oracleHeaders(h map[string][]string) string {
	if h == nil {
		return ""
	}
	var parts []string
	for _, name := range []string{"Etag", "Last-Modified"} {
		if v, ok := h[name]; ok && len(v) > 0 {
			parts = append(parts, name+"="+strings.Join(v, ","))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
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
