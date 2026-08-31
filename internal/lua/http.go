// Lua bindings for the http.* module.
//
// Provides HTTP client capabilities for Lua scripts to call external APIs.
// Follows the same error convention as the ai.* module:
//
//	expected runtime failure  -> (nil, err_table)
//	programming error         -> RaiseError
//
// The error table mirrors ai.Error so scripts switching between ai.chat
// and http.request see the same shape: kind (string), message (string),
// retry_after (number, always 0 for http), details (string, unwrapped
// cause when present). Scripts branch on err.kind.
//
// Error kinds:
//   - timeout:      request exceeded deadline
//   - canceled:     request was canceled (e.g., runtime shutting down)
//   - network:      DNS, connection refused, TLS, read error, etc.
//   - bad_response: response body exceeded the 10 MiB cap
//
// # Request encodings
//
// Three ways to supply a request body, all provider-neutral:
//
//	body       = "..."                        -- raw bytes; you set Content-Type
//	form       = {k = "v"}                    -- multipart/form-data
//	basic_auth = {user = "api", pass = key}   -- Authorization: Basic
//
// form and basic_auth exist because a JSON-only client is not a general HTTP
// client. Several widely used APIs (Mailgun's send endpoint being the case
// that forced this) accept multipart/form-data with HTTP Basic and nothing
// else, so without them "call any API from Lua" quietly meant "call any JSON
// API from Lua". They are deliberately NOT mail-specific: form encoding and
// Basic auth are HTTP, and putting them here rather than behind a mail
// abstraction is what lets the next form-encoded upstream be reached without
// another Go change.
//
// JSON encode/decode helpers live separately under rela.json (see json.go).
package lua

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// httpMaxResponseBytes caps the response body to prevent OOM.
const httpMaxResponseBytes = 10 * 1024 * 1024 // 10 MiB

// httpDefaultTimeout is the hard ceiling for HTTP requests when the
// script does not specify a per-request timeout.
const httpDefaultTimeout = 30 * time.Second

// newHTTPClient creates the shared HTTP client used by the http module.
// Redirect following is disabled so scripts handle redirects explicitly.
//
// The Transport is explicitly bounded rather than reusing
// http.DefaultTransport: rela's scheduler and MCP server are
// long-running processes, and a script that fans out to many distinct
// hosts would otherwise grow idle connections without bound.
// MaxResponseHeaderBytes also caps header memory before the 10 MiB body
// cap fires, defending against a hostile server returning thousands of
// huge headers.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: httpDefaultTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:                  http.ProxyFromEnvironment,
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			MaxConnsPerHost:        50,
			IdleConnTimeout:        90 * time.Second,
			TLSHandshakeTimeout:    10 * time.Second,
			ExpectContinueTimeout:  1 * time.Second,
			ResponseHeaderTimeout:  30 * time.Second,
			MaxResponseHeaderBytes: 1 << 20, // 1 MiB
			ForceAttemptHTTP2:      true,
		},
	}
}

// httpClient is the shared HTTP client for all Lua HTTP requests within
// a process. Connection pooling is reused across requests. Process-global
// rather than per-Runtime so that connection-pooling benefits hold even
// when scripts run in short-lived runtimes (CLI script invocations,
// scheduler ticks, MCP tool calls).
var httpClient = newHTTPClient()

// httpBindings implements the http.* module bindings: request plus the
// per-method convenience functions. A type of its own rather than more
// methods on [Runtime] (the urlHelpers rationale in urls.go): these
// functions need nothing from the runtime — no store, no deps, no
// capabilities, not even the Lua state, since each receives the
// *lua.LState it is called with and the request context comes from that
// state (see httpContext). The capability gate stays where it was:
// registerBindings on Runtime registers this module only when caps.HTTP
// was granted.
//
// Nil: the zero value is ready to use; it holds no state to initialize.
type httpBindings struct{}

// registerHTTPModule installs the top-level `http` global with `request`
// plus the per-method convenience functions (get, post, put, patch, delete).
// JSON helpers live separately under rela.json (see json.go).
func (r *Runtime) registerHTTPModule() {
	h := httpBindings{}
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "request", r.L.NewFunction(h.luaHTTPRequest))
	r.L.SetField(tbl, "get", r.L.NewFunction(h.luaHTTPGet))
	r.L.SetField(tbl, "post", r.L.NewFunction(h.luaHTTPPost))
	r.L.SetField(tbl, "put", r.L.NewFunction(h.luaHTTPPut))
	r.L.SetField(tbl, "patch", r.L.NewFunction(h.luaHTTPPatch))
	r.L.SetField(tbl, "delete", r.L.NewFunction(h.luaHTTPDelete))
	r.L.SetGlobal("http", tbl)
}

// luaHTTPRequest implements http.request(opts) where opts is a table with:
//
//	url        (string, required)
//	method     (string, optional, default "GET")
//	headers    (table, optional)
//	body       (string, optional)
//	form       (table, optional, string->string; multipart/form-data)
//	basic_auth (table, optional, {user=..., pass=...})
//	timeout    (number, optional, seconds)
//
// Returns (response_table, nil) on success, (nil, err_table) on failure.
func (h httpBindings) luaHTTPRequest(ls *lua.LState) int {
	opts := ls.CheckTable(1)
	parsed, err := parseHTTPRequestOpts(opts)
	if err != nil {
		ls.RaiseError("http.request: %s", err.Error())
		return 0
	}
	return h.doHTTPRequest(ls, "http.request", parsed)
}

// luaHTTPGet implements http.get(url, opts?) -> (response, nil) | (nil, err).
func (h httpBindings) luaHTTPGet(ls *lua.LState) int {
	return h.luaHTTPSimple(ls, "http.get", http.MethodGet, false)
}

// luaHTTPPost implements http.post(url, body, opts?) -> (response, nil) | (nil, err).
func (h httpBindings) luaHTTPPost(ls *lua.LState) int {
	return h.luaHTTPSimple(ls, "http.post", http.MethodPost, true)
}

// luaHTTPPut implements http.put(url, body, opts?) -> (response, nil) | (nil, err).
func (h httpBindings) luaHTTPPut(ls *lua.LState) int {
	return h.luaHTTPSimple(ls, "http.put", http.MethodPut, true)
}

// luaHTTPPatch implements http.patch(url, body, opts?) -> (response, nil) | (nil, err).
func (h httpBindings) luaHTTPPatch(ls *lua.LState) int {
	return h.luaHTTPSimple(ls, "http.patch", http.MethodPatch, true)
}

// luaHTTPDelete implements http.delete(url, opts?) -> (response, nil) | (nil, err).
func (h httpBindings) luaHTTPDelete(ls *lua.LState) int {
	return h.luaHTTPSimple(ls, "http.delete", http.MethodDelete, false)
}

// luaHTTPSimple implements the convenience-method shape:
// - position 1: URL string
// - position 2 (when withBody): body string
// - last position: optional opts table {headers, timeout, form, basic_auth}
//
// fnName ("http.get", etc.) is used as the prefix on raised errors so
// scripts see the entry-point name in error messages, not "http.request".
func (h httpBindings) luaHTTPSimple(ls *lua.LState, fnName, method string, withBody bool) int {
	rawURL := ls.CheckString(1)
	body := ""
	optsPos := 2
	if withBody {
		body = ls.OptString(2, "")
		optsPos = 3
	}
	opts, err := parseConvenienceOpts(ls, optsPos)
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}
	opts.method = method
	opts.url = reqURL
	opts.body = body
	return h.doHTTPRequest(ls, fnName, opts)
}

// doHTTPRequest performs the actual HTTP request and pushes the result
// onto the Lua stack. Returns the number of values pushed (always 2).
// fnName is used as the prefix on raised errors so scripts see the
// entry-point name (e.g. "http.get") rather than always "http.request".
//
// Takes the parsed opts STRUCT rather than seven positional arguments: the
// convenience methods and http.request now agree on a growing set of request
// features (body, form, basic auth), and a parameter list that grows with each
// one is how a caller ends up passing headers where the body goes.
func (h httpBindings) doHTTPRequest(ls *lua.LState, fnName string, o httpRequestOpts) int {
	ctx := httpContext(ls)
	if o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}

	bodyReader, contentType, err := o.payload()
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}

	httpReq, err := http.NewRequestWithContext(ctx, o.method, o.url.String(), bodyReader)
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}

	for k, v := range o.headers {
		httpReq.Header.Set(k, v)
	}
	// Content-Type is set AFTER the caller's headers so the multipart boundary,
	// which only this function knows, cannot be clobbered by a script that
	// helpfully set `Content-Type = "multipart/form-data"` without one. A body
	// whose declared boundary does not match its parts is unparseable at the
	// far end and the failure reads as "the server rejected my request".
	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	if o.basicAuth != nil {
		httpReq.SetBasicAuth(o.basicAuth.user, o.basicAuth.pass)
	}

	resp, doErr := httpClient.Do(httpReq)
	if doErr != nil {
		return pushHTTPError(ls, classifyHTTPError(doErr))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := readHTTPBody(resp.Body)
	if readErr != nil {
		return pushHTTPError(ls, readErr)
	}

	return pushHTTPResponse(ls, resp, respBody)
}

// httpContext returns the context for HTTP calls, propagating the
// Lua state's context (for timeout) or falling back to Background.
func httpContext(ls *lua.LState) context.Context {
	if ctx := ls.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// httpRequestOpts is the parsed form of the opts table passed to http.request.
type httpRequestOpts struct {
	method  string
	url     *url.URL
	headers map[string]string
	body    string

	// form, when non-nil, makes the request a multipart/form-data POST body
	// built from these fields. Mutually exclusive with body — see payload.
	form []formField

	// basicAuth, when non-nil, sets an Authorization: Basic header.
	basicAuth *basicAuthOpts

	timeout time.Duration
}

// formField is one multipart/form-data part. A SLICE of these rather than a
// map because Mailgun and friends accept repeated field names (`to` given
// three times is three recipients), which a map cannot express — and because
// part order is then deterministic, so a test can assert on the body it
// produced rather than on a set.
type formField struct {
	name  string
	value string
}

// basicAuthOpts carries an HTTP Basic credential.
//
// A struct with two named fields rather than a "user:pass" string: the colon
// is significant in Basic auth, so a password containing one would silently
// split in the wrong place, and the resulting request would fail
// authentication with no hint as to why.
type basicAuthOpts struct {
	user string
	pass string
}

// payload returns the request body, its Content-Type (empty when the caller's
// headers should decide), and any error.
//
// Body and form are mutually exclusive and that is an ERROR rather than a
// precedence rule. Silently preferring one would mean a script that set both —
// most plausibly by adding `form` to a call that already had `body` — sends a
// request missing half of what its author intended, and gets a 400 from the
// far end describing a field problem rather than a local mistake.
func (o httpRequestOpts) payload() (io.Reader, string, error) {
	if o.form != nil && o.body != "" {
		return nil, "", errors.New("body and form are mutually exclusive; set one")
	}
	if o.form == nil {
		if o.body == "" {
			return nil, "", nil
		}
		return strings.NewReader(o.body), "", nil
	}

	// Buffered rather than streamed through an io.Pipe. The whole body is
	// already in memory — it came from Lua strings — so a pipe would add a
	// goroutine and an error-propagation path to move bytes that are sitting
	// right there. It also means multipart.Writer.Close, and therefore the
	// closing boundary, is done before the request is built rather than
	// racing it.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range o.form {
		if err := w.WriteField(f.name, f.value); err != nil {
			return nil, "", fmt.Errorf("form field %q: %w", f.name, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("finalizing form: %w", err)
	}
	return &buf, w.FormDataContentType(), nil
}

// parseHTTPRequestOpts extracts fields from the opts table for http.request().
func parseHTTPRequestOpts(opts *lua.LTable) (httpRequestOpts, error) {
	var out httpRequestOpts

	// url (required)
	urlVal := opts.RawGetString("url")
	urlStr, ok := urlVal.(lua.LString)
	if !ok || urlStr == "" {
		return out, errors.New("url must be a non-empty string")
	}

	reqURL, err := validateURL(string(urlStr))
	if err != nil {
		return out, err
	}
	out.url = reqURL

	// method (optional, default GET)
	out.method = http.MethodGet
	if v := opts.RawGetString("method"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return out, errors.New("method must be a string")
		}
		out.method = strings.ToUpper(string(s))
		if err := validateHTTPMethod(out.method); err != nil {
			return out, err
		}
	}

	// headers (optional)
	out.headers = make(map[string]string)
	if v := opts.RawGetString("headers"); v != lua.LNil {
		headers, err := parseHeaderTable(v)
		if err != nil {
			return out, err
		}
		out.headers = headers
	}

	// body (optional)
	if v := opts.RawGetString("body"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return out, errors.New("body must be a string")
		}
		out.body = string(s)
	}

	// form (optional): multipart/form-data fields.
	if v := opts.RawGetString("form"); v != lua.LNil {
		fields, err := parseFormFields(v)
		if err != nil {
			return out, err
		}
		out.form = fields
	}

	// basic_auth (optional)
	if v := opts.RawGetString("basic_auth"); v != lua.LNil {
		auth, err := parseBasicAuth(v)
		if err != nil {
			return out, err
		}
		out.basicAuth = auth
	}

	// timeout (optional, seconds)
	if v := opts.RawGetString("timeout"); v != lua.LNil {
		n, ok := v.(lua.LNumber)
		if !ok {
			return out, errors.New("timeout must be a number")
		}
		if n <= 0 {
			return out, errors.New("timeout must be positive")
		}
		out.timeout = time.Duration(float64(n) * float64(time.Second))
	}

	return out, nil
}

// parseFormFields converts the `form` table into ordered multipart parts.
//
// Two shapes are accepted, because the two things a caller wants are genuinely
// different:
//
//	form = { subject = "hi", to = "a@example.com" }        -- one value per name
//	form = { {"to", "a@example.com"}, {"to", "b@example.com"} }  -- repeats
//
// The map shape is what almost every call wants and reads best. The array
// shape exists because form encoding permits repeated names and several
// providers rely on it (Mailgun takes multiple `to` fields), which a Lua table
// keyed by name cannot express at all.
//
// Map iteration order is not defined, so the map shape is SORTED by name.
// Unsorted, the produced body would differ between runs and every test
// asserting on it would be flaky — and a caller who needs a specific order
// has the array shape.
func parseFormFields(v lua.LValue) ([]formField, error) {
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("form must be a table, got %s", v.Type().String())
	}

	// A non-zero array part means the positional shape. Len() reports the
	// array part only, so a pure string-keyed table reads 0 here.
	if n := tbl.Len(); n > 0 {
		out := make([]formField, 0, n)
		for i := 1; i <= n; i++ {
			pair, pok := tbl.RawGetInt(i).(*lua.LTable)
			if !pok || pair.Len() != 2 {
				return nil, fmt.Errorf("form entry %d must be a {name, value} pair", i)
			}
			name, nok := pair.RawGetInt(1).(lua.LString)
			val, vok := pair.RawGetInt(2).(lua.LString)
			if !nok || !vok {
				return nil, fmt.Errorf("form entry %d must be a {string, string} pair", i)
			}
			if name == "" {
				return nil, fmt.Errorf("form entry %d has an empty field name", i)
			}
			out = append(out, formField{name: string(name), value: string(val)})
		}
		return out, nil
	}

	var out []formField
	var fieldErr error
	tbl.ForEach(func(k, v lua.LValue) {
		if fieldErr != nil {
			return
		}
		ks, kok := k.(lua.LString)
		if !kok {
			fieldErr = fmt.Errorf("form field name must be a string, got %s", k.Type().String())
			return
		}
		vs, vok := v.(lua.LString)
		if !vok {
			// Numbers are refused rather than coerced. Lua has one numeric
			// type, so 1 formats as "1" and 1.0 as "1" too — a caller passing
			// a computed value gets a string whose shape depends on float
			// rounding. tostring() at the call site is explicit about which
			// rendering was meant.
			fieldErr = fmt.Errorf("form field %q must be a string, got %s (use tostring)",
				string(ks), v.Type().String())
			return
		}
		out = append(out, formField{name: string(ks), value: string(vs)})
	})
	if fieldErr != nil {
		return nil, fieldErr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

// parseBasicAuth converts the `basic_auth` table into a credential.
//
// An empty user is refused: `basic_auth = {pass = secret}` with a mistyped
// user key would otherwise send `Authorization: Basic OnNlY3JldA==` — a
// syntactically valid header with an empty username — and the server's 401
// would say nothing about the typo. An empty PASS is allowed, because APIs
// that authenticate with a token-as-username and no password are common
// enough (Mailgun's own `api:KEY` is the mirror image of it).
func parseBasicAuth(v lua.LValue) (*basicAuthOpts, error) {
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("basic_auth must be a table, got %s", v.Type().String())
	}
	user, uok := tbl.RawGetString("user").(lua.LString)
	if !uok || user == "" {
		return nil, errors.New("basic_auth.user must be a non-empty string")
	}
	pass, _ := tbl.RawGetString("pass").(lua.LString)
	return &basicAuthOpts{user: string(user), pass: string(pass)}, nil
}

// parseConvenienceOpts extracts the shared option table used by the
// convenience methods (get, post, ...): headers, timeout, form and
// basic_auth. Returns a partially-filled httpRequestOpts — the caller fills in
// method, url and body, which come from positional arguments.
//
// Returns an error rather than raising so the caller can prefix the function
// name (http.get / http.post / ...) — matches parseHTTPRequestOpts.
func parseConvenienceOpts(ls *lua.LState, pos int) (httpRequestOpts, error) {
	out := httpRequestOpts{headers: make(map[string]string)}

	optsTbl := ls.OptTable(pos, nil)
	if optsTbl == nil {
		return out, nil
	}

	if v := optsTbl.RawGetString("headers"); v != lua.LNil {
		headers, err := parseHeaderTable(v)
		if err != nil {
			return httpRequestOpts{}, err
		}
		out.headers = headers
	}

	if v := optsTbl.RawGetString("form"); v != lua.LNil {
		fields, err := parseFormFields(v)
		if err != nil {
			return httpRequestOpts{}, err
		}
		out.form = fields
	}

	if v := optsTbl.RawGetString("basic_auth"); v != lua.LNil {
		auth, err := parseBasicAuth(v)
		if err != nil {
			return httpRequestOpts{}, err
		}
		out.basicAuth = auth
	}

	if v := optsTbl.RawGetString("timeout"); v != lua.LNil {
		n, ok := v.(lua.LNumber)
		if !ok {
			return httpRequestOpts{}, fmt.Errorf("timeout must be a number, got %s", v.Type().String())
		}
		if n <= 0 {
			return httpRequestOpts{}, errors.New("timeout must be positive")
		}
		out.timeout = time.Duration(float64(n) * float64(time.Second))
	}

	return out, nil
}

// parseHeaderTable converts a Lua table of string->string into a header map.
//
// Shared by http.request and the convenience methods so the two cannot drift
// on what they accept — they had separate copies of this loop, which is
// exactly how one of them ends up quietly permitting a numeric value.
func parseHeaderTable(v lua.LValue) (map[string]string, error) {
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("headers must be a table, got %s", v.Type().String())
	}
	headers := make(map[string]string)
	var headerErr error
	tbl.ForEach(func(k, v lua.LValue) {
		if headerErr != nil {
			return
		}
		ks, kok := k.(lua.LString)
		if !kok {
			headerErr = fmt.Errorf("header key must be a string, got %s", k.Type().String())
			return
		}
		vs, vok := v.(lua.LString)
		if !vok {
			headerErr = fmt.Errorf("header value for %q must be a string, got %s", string(ks), v.Type().String())
			return
		}
		headers[string(ks)] = string(vs)
	})
	if headerErr != nil {
		return nil, headerErr
	}
	return headers, nil
}

// validateURL parses and validates a URL for HTTP requests.
func validateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", err.Error())
	}
	switch u.Scheme {
	case "http", "https":
		// ok
	case "":
		return nil, errors.New("URL must have http or https scheme")
	default:
		return nil, fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("URL must have a host")
	}
	// Reject userinfo (http://user:pass@host/...) — accepting it would
	// silently send Basic Auth and any logging of the URL would leak the
	// credentials. Scripts that need Basic Auth should set the
	// Authorization header explicitly.
	if u.User != nil {
		return nil, errors.New("URL must not contain userinfo; set the Authorization header instead")
	}
	return u, nil
}

// validateHTTPMethod rejects methods that http.NewRequest would reject later
// (anything containing invalid characters or whitespace). The method is
// assumed already-uppercased.
func validateHTTPMethod(m string) error {
	if m == "" {
		return errors.New("method must not be empty")
	}
	// RFC 7230 token: 1*tchar, tchar = ALPHA / DIGIT / "!#$%&'*+-.^_`|~"
	for i := range len(m) {
		c := m[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			// ok
		case c == '!' || c == '#' || c == '$' || c == '%' || c == '&' || c == '\'' ||
			c == '*' || c == '+' || c == '-' || c == '.' || c == '^' || c == '_' ||
			c == '`' || c == '|' || c == '~':
			// ok
		default:
			return fmt.Errorf("method contains invalid character %q", c)
		}
	}
	return nil
}

// httpError represents an HTTP-level error surfaced to Lua scripts.
// The Cause field is surfaced to scripts as err.details (matching the ai
// module's shape), letting scripts inspect low-level transport errors.
//
// There is no Status field: HTTP-level errors arise from the transport
// (no response received) or the body (capped). A non-2xx response is not
// an error — it returns a normal response with status_code populated.
type httpError struct {
	Kind    string
	Message string
	Cause   error
}

// classifyHTTPError converts a net/http client error into an httpError.
func classifyHTTPError(err error) *httpError {
	if err == nil {
		return nil
	}
	msg := err.Error()

	if errors.Is(err, context.DeadlineExceeded) {
		return &httpError{Kind: "timeout", Message: msg, Cause: err}
	}
	if errors.Is(err, context.Canceled) {
		return &httpError{Kind: "canceled", Message: msg, Cause: err}
	}

	// Client-level timeout (http.Client.Timeout) surfaces as a *url.Error
	// whose Timeout() is true but does not wrap context.DeadlineExceeded.
	// Keep this branch distinct from the errors.Is check above.
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return &httpError{Kind: "timeout", Message: msg, Cause: err}
	}

	return &httpError{Kind: "network", Message: msg, Cause: err}
}

// errHTTPBodyTooLarge is returned when the response exceeds httpMaxResponseBytes.
var errHTTPBodyTooLarge = errors.New("response body exceeded 10 MiB limit")

// readHTTPBody reads up to httpMaxResponseBytes from the response body.
// Read errors are routed through classifyHTTPError so a mid-stream
// context.DeadlineExceeded surfaces as kind="timeout" rather than
// silently bucketing all read failures as "network".
func readHTTPBody(r io.Reader) ([]byte, *httpError) {
	limited := io.LimitReader(r, httpMaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		classified := classifyHTTPError(err)
		// Prefix message so the operator can tell the failure happened
		// during body read rather than connect/handshake.
		classified.Message = "reading response body: " + classified.Message
		return nil, classified
	}
	if int64(len(body)) > httpMaxResponseBytes {
		return nil, &httpError{
			Kind:    "bad_response",
			Message: errHTTPBodyTooLarge.Error(),
			Cause:   errHTTPBodyTooLarge,
		}
	}
	return body, nil
}

// pushHTTPError pushes (nil, err_table) onto the Lua stack. Fields:
// kind (string), message (string), retry_after (number, always 0 for
// http — present for ai-shape parity), details (string, unwrapped cause
// when present). status is intentionally absent: HTTP-level errors carry
// no status code (transport failed before a response, or the body was
// over the cap). A non-2xx response is returned as a normal response
// table with status_code populated, not as an error.
func pushHTTPError(ls *lua.LState, e *httpError) int {
	ls.Push(lua.LNil)
	tbl := ls.NewTable()
	tbl.RawSetString("kind", lua.LString(e.Kind))
	tbl.RawSetString("message", lua.LString(e.Message))
	tbl.RawSetString("retry_after", lua.LNumber(0))
	if e.Cause != nil {
		tbl.RawSetString("details", lua.LString(e.Cause.Error()))
	} else {
		tbl.RawSetString("details", lua.LString(""))
	}
	ls.Push(tbl)
	return 2
}

// pushHTTPResponse pushes a response table onto the Lua stack.
// The response table has: status_code (number), status (string),
// headers (table), body (string).
func pushHTTPResponse(ls *lua.LState, resp *http.Response, body []byte) int {
	tbl := ls.NewTable()
	tbl.RawSetString("status_code", lua.LNumber(resp.StatusCode))
	tbl.RawSetString("status", lua.LString(resp.Status))

	headersTbl := ls.NewTable()
	for name, values := range resp.Header {
		if len(values) > 0 {
			headersTbl.RawSetString(strings.ToLower(name), lua.LString(values[0]))
		}
	}
	tbl.RawSetString("headers", headersTbl)
	tbl.RawSetString("body", lua.LString(string(body)))

	ls.Push(tbl)
	ls.Push(lua.LNil)
	return 2
}
