package apptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// HTTP stubs (TKT-HB313W5). Every outbound HTTP call of a case goes through
// the case's stubTransport: Lua http.* in automations, actions and flow job
// scripts, and the POST of a flow `kind: webhook` job. A call that a stub
// matches gets the stub's canned answer. Any other call fails the case and
// gets a network error, so a script that swallows the error cannot pass.
// Nothing reaches the network.

// maxStubBody bounds how much of a request body is read and recorded.
const maxStubBody = 10 << 20

// Error kinds a stubbed response may simulate. They reach a script as the
// err.kind of the http.* error table.
const (
	stubErrTimeout = "timeout"
	stubErrNetwork = "network"
)

// stubTimeoutError is the error of a stubbed timeout. It is a net.Error with
// Timeout() true, the shape the Lua binding classifies as "timeout".
type stubTimeoutError struct{}

func (stubTimeoutError) Error() string   { return "stubbed timeout (t.http.stub)" }
func (stubTimeoutError) Timeout() bool   { return true }
func (stubTimeoutError) Temporary() bool { return true }

// errStubNetwork is the error of a stubbed network failure.
var errStubNetwork = errors.New("stubbed network error (t.http.stub)")

// errUnmatched is what an unmatched call fails with. The case fails too.
var errUnmatched = errors.New("no t.http.stub matches this request; rela test makes no real HTTP calls")

// stubResponse is one canned answer: a response, or an error kind.
type stubResponse struct {
	status int
	header http.Header
	body   []byte
	// errKind is "timeout" or "network"; empty for a response.
	errKind string
}

// httpStub is one t.http.stub registration.
type httpStub struct {
	name string
	// named is set when the test gave the name; a generated name is the
	// stub's description, so a failure message does not repeat it.
	named bool
	where string
	// method is upper case; empty matches every method.
	method string
	// Exactly one of url, prefix and pattern is set.
	url     *url.URL
	prefix  string
	pattern *regexp.Regexp
	// header maps canonical header names to the exact value each must have.
	header map[string]string
	// body, when set, is the exact request body.
	body *string
	// json, when set, must be a subset of the request's JSON body.
	json any
	// form, when set, must be a subset of the request's form fields.
	form map[string]string

	// responses are given in order. When repeat is set the last one is
	// given again for every further call; otherwise the stub is spent once
	// every response was given.
	responses []stubResponse
	repeat    bool
	// calls are the indexes of the calls the stub answered, in
	// [stubTransport.calls].
	calls []int
}

// describe is the stub as a failure message lists it.
func (s *httpStub) describe() string {
	method := s.method
	if method == "" {
		method = "*"
	}
	var target string
	switch {
	case s.url != nil:
		target = s.url.String()
	case s.prefix != "":
		target = s.prefix + "*"
	default:
		target = "~" + s.pattern.String()
	}
	var extra []string
	if len(s.header) > 0 {
		extra = append(extra, "headers "+strings.Join(slices.Sorted(maps.Keys(s.header)), ", "))
	}
	if s.body != nil {
		extra = append(extra, "body")
	}
	if s.json != nil {
		extra = append(extra, "json")
	}
	if s.form != nil {
		extra = append(extra, "form")
	}
	if !s.repeat {
		extra = append(extra, fmt.Sprintf("%d of %d responses given", len(s.calls), len(s.responses)))
	}
	out := method + " " + target
	if s.named {
		out = fmt.Sprintf("%q: %s", s.name, out)
	}
	if len(extra) > 0 {
		out += " (" + strings.Join(extra, "; ") + ")"
	}
	return out + " at " + s.where
}

// spent reports whether a sequence stub has given every response.
func (s *httpStub) spent() bool { return !s.repeat && len(s.calls) >= len(s.responses) }

// next is the response for the stub's next call.
func (s *httpStub) next() stubResponse {
	i := min(len(s.calls), len(s.responses)-1)
	return s.responses[i]
}

// recordedCall is one request the case's code sent.
type recordedCall struct {
	method string
	url    string
	header http.Header
	body   []byte
	// stub names the stub that answered; empty for an unmatched call.
	stub string
}

// stubTransport is the HTTP transport of one case. Safe for concurrent use:
// a world's background workers may send while the case registers stubs.
type stubTransport struct {
	mu    sync.Mutex
	stubs []*httpStub
	calls []recordedCall
	// failure is the case's first unmatched call.
	failure *failure
	// where, when set, names the test file line running when a call
	// fails. It is called on the case's goroutine, which is the only one
	// that sends requests in a case.
	where func() string
}

// add registers s. An explicit name already taken in this case is an
// error, so stub:calls() and the failure messages stay unambiguous; a
// generated name gets a number instead.
func (t *stubTransport) add(s *httpStub) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	base := s.name
	for n := 2; ; n++ {
		i := slices.IndexFunc(t.stubs, func(o *httpStub) bool { return o.name == s.name })
		if i < 0 {
			break
		}
		if s.named {
			return fmt.Errorf("t.http.stub: a stub named %q is already registered at %s", s.name, t.stubs[i].where)
		}
		s.name = fmt.Sprintf("%s #%d", base, n)
	}
	t.stubs = append(t.stubs, s)
	return nil
}

// RoundTrip answers req from the first registered stub that matches it and
// is not spent, and records the call.
func (t *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	body, berr := readRequestBody(req)
	t.mu.Lock()
	defer t.mu.Unlock()
	call := recordedCall{method: req.Method, url: req.URL.String(), header: req.Header.Clone(), body: body}
	if berr != nil {
		// Recorded and failed, so a script that swallows the error
		// cannot pass.
		t.calls = append(t.calls, call)
		t.fail(fmt.Sprintf("HTTP call %s %s: %v", req.Method, req.URL.String(), berr))
		return nil, berr
	}
	var spent *httpStub
	for _, s := range t.stubs {
		if !s.matches(req, body) {
			continue
		}
		if s.spent() {
			if spent == nil {
				spent = s
			}
			continue
		}
		call.stub = s.name
		r := s.next()
		s.calls = append(s.calls, len(t.calls))
		t.calls = append(t.calls, call)
		return r.answer(req)
	}
	t.calls = append(t.calls, call)
	t.unmatched(req, spent)
	return nil, errUnmatched
}

// unmatched records the case's failure for req, which no stub answered.
// spent, when set, is a stub that matched but had given all its responses.
func (t *stubTransport) unmatched(req *http.Request, spent *httpStub) {
	if t.failure != nil {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "unexpected HTTP call %s %s", req.Method, req.URL.String())
	switch {
	case spent != nil:
		fmt.Fprintf(&b, ": stub %q matches it but has given all its %d responses", spent.name, len(spent.responses))
	case len(t.stubs) == 0:
		b.WriteString("; no stubs are registered. Register one with t.http.stub{...}")
	default:
		b.WriteString("; registered stubs:")
		for _, s := range t.stubs {
			b.WriteString("\n  - " + s.describe())
		}
	}
	t.fail(b.String())
}

// fail records msg as the case's failure unless one is recorded.
func (t *stubTransport) fail(msg string) {
	if t.failure != nil {
		return
	}
	t.failure = &failure{msg: msg}
	if t.where != nil {
		t.failure.where = t.where()
	}
}

// takeFailure returns the recorded unmatched call, if any.
func (t *stubTransport) takeFailure() *failure {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failure
}

// recorded returns a copy of the calls, or of the calls stub answered when
// stub is not nil.
func (t *stubTransport) recorded(stub *httpStub) []recordedCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	if stub == nil {
		return slices.Clone(t.calls)
	}
	out := make([]recordedCall, 0, len(stub.calls))
	for _, i := range stub.calls {
		out = append(out, t.calls[i])
	}
	return out
}

// readRequestBody reads and restores req's body.
func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	defer func() { _ = req.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(req.Body, maxStubBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxStubBody {
		return nil, fmt.Errorf("the request body is larger than %d bytes, which rela test does not take", maxStubBody)
	}
	return body, nil
}

// answer is r as the transport's result for req.
func (r stubResponse) answer(req *http.Request) (*http.Response, error) {
	switch r.errKind {
	case stubErrTimeout:
		return nil, stubTimeoutError{}
	case stubErrNetwork:
		return nil, errStubNetwork
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", r.status, http.StatusText(r.status)),
		StatusCode:    r.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        r.header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(r.body)),
		ContentLength: int64(len(r.body)),
		Request:       req,
	}, nil
}

// matches reports whether req, with body, meets every condition of s.
func (s *httpStub) matches(req *http.Request, body []byte) bool {
	if s.method != "" && s.method != req.Method {
		return false
	}
	if !s.matchesURL(req.URL) {
		return false
	}
	for name, want := range s.header {
		if req.Header.Get(name) != want {
			return false
		}
	}
	if s.body != nil && *s.body != string(body) {
		return false
	}
	if s.json != nil {
		var got any
		if err := json.Unmarshal(body, &got); err != nil || !jsonSubset(s.json, got) {
			return false
		}
	}
	if s.form != nil && !formSubset(s.form, req, body) {
		return false
	}
	return true
}

// matchesURL compares u with the stub's url, prefix or pattern. An exact
// url ignores the order of the query parameters.
func (s *httpStub) matchesURL(u *url.URL) bool {
	full := u.String()
	switch {
	case s.url != nil:
		return u.Scheme == s.url.Scheme && sameHost(u, s.url) && cleanPath(u) == cleanPath(s.url) &&
			reflect.DeepEqual(u.Query(), s.url.Query())
	case s.prefix != "":
		return strings.HasPrefix(full, s.prefix)
	default:
		return s.pattern.MatchString(full)
	}
}

// sameHost compares the hosts of a and b, which have the same scheme,
// ignoring case and a default port.
func sameHost(a, b *url.URL) bool {
	return strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// port is u's port, or the scheme's default one.
func port(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch u.Scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	return ""
}

// cleanPath is u's path, with "/" for an empty one.
func cleanPath(u *url.URL) string {
	if u.Path == "" {
		return "/"
	}
	return u.EscapedPath()
}

// jsonSubset reports whether want is a subset of got: every key of a want
// object is in got with a subset value, a want array has as many elements
// as got and each is a subset, and a scalar is equal. An empty want object
// or array matches any object or array, because Lua cannot tell {} as an
// object from {} as an array. Numbers compare exactly: both sides are
// float64, as Lua numbers are.
func jsonSubset(want, got any) bool {
	switch w := want.(type) {
	case map[string]any:
		if len(w) == 0 {
			return isContainer(got)
		}
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, wv := range w {
			gv, present := g[k]
			if !present || !jsonSubset(wv, gv) {
				return false
			}
		}
		return true
	case []any:
		if len(w) == 0 {
			return isContainer(got)
		}
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !jsonSubset(w[i], g[i]) {
				return false
			}
		}
		return true
	case float64:
		g, ok := got.(float64)
		return ok && g == w
	default:
		return reflect.DeepEqual(want, got)
	}
}

func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// formSubset reports whether the request's form body, urlencoded or
// multipart, holds every field of want with that value (the first value
// when the field repeats).
func formSubset(want map[string]string, req *http.Request, body []byte) bool {
	fields, ok := formFields(req, body)
	if !ok {
		return false
	}
	for k, v := range want {
		got, present := fields[k]
		if !present || len(got) == 0 || got[0] != v {
			return false
		}
	}
	return true
}

// formFields parses body as req's Content-Type says.
func formFields(req *http.Request, body []byte) (url.Values, bool) {
	mt, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return nil, false
	}
	switch mt {
	case "application/x-www-form-urlencoded":
		v, err := url.ParseQuery(string(body))
		return v, err == nil
	case "multipart/form-data":
		r := req.Clone(req.Context())
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err := r.ParseMultipartForm(maxStubBody); err != nil {
			return nil, false
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()
		return url.Values(r.MultipartForm.Value), true
	}
	return nil, false
}

// decodedJSON is the body decoded as JSON, or nil when it is not JSON.
func (c recordedCall) decodedJSON() any {
	if len(c.body) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(c.body, &v); err != nil {
		return nil
	}
	return v
}
