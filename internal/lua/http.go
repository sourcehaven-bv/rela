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
// JSON encode/decode helpers live separately under rela.json (see json.go).
package lua

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

// registerHTTPModule installs the top-level `http` global with `request`
// plus the per-method convenience functions (get, post, put, patch, delete).
// JSON helpers live separately under rela.json (see json.go).
func (r *Runtime) registerHTTPModule() {
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "request", r.L.NewFunction(r.luaHTTPRequest))
	r.L.SetField(tbl, "get", r.L.NewFunction(r.luaHTTPGet))
	r.L.SetField(tbl, "post", r.L.NewFunction(r.luaHTTPPost))
	r.L.SetField(tbl, "put", r.L.NewFunction(r.luaHTTPPut))
	r.L.SetField(tbl, "patch", r.L.NewFunction(r.luaHTTPPatch))
	r.L.SetField(tbl, "delete", r.L.NewFunction(r.luaHTTPDelete))
	r.L.SetGlobal("http", tbl)
}

// luaHTTPRequest implements http.request(opts) where opts is a table with:
//
//	url      (string, required)
//	method   (string, optional, default "GET")
//	headers  (table, optional)
//	body     (string, optional)
//	timeout  (number, optional, seconds)
//
// Returns (response_table, nil) on success, (nil, err_table) on failure.
func (r *Runtime) luaHTTPRequest(ls *lua.LState) int {
	opts := ls.CheckTable(1)
	parsed, err := parseHTTPRequestOpts(opts)
	if err != nil {
		ls.RaiseError("http.request: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, "http.request", parsed.method, parsed.url, parsed.headers, parsed.body, parsed.timeout)
}

// luaHTTPGet implements http.get(url, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPGet(ls *lua.LState) int {
	return r.luaHTTPSimple(ls, "http.get", http.MethodGet, false)
}

// luaHTTPPost implements http.post(url, body, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPPost(ls *lua.LState) int {
	return r.luaHTTPSimple(ls, "http.post", http.MethodPost, true)
}

// luaHTTPPut implements http.put(url, body, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPPut(ls *lua.LState) int {
	return r.luaHTTPSimple(ls, "http.put", http.MethodPut, true)
}

// luaHTTPPatch implements http.patch(url, body, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPPatch(ls *lua.LState) int {
	return r.luaHTTPSimple(ls, "http.patch", http.MethodPatch, true)
}

// luaHTTPDelete implements http.delete(url, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPDelete(ls *lua.LState) int {
	return r.luaHTTPSimple(ls, "http.delete", http.MethodDelete, false)
}

// luaHTTPSimple implements the convenience-method shape:
// - position 1: URL string
// - position 2 (when withBody): body string
// - last position: optional opts table {headers, timeout}
//
// fnName ("http.get", etc.) is used as the prefix on raised errors so
// scripts see the entry-point name in error messages, not "http.request".
func (r *Runtime) luaHTTPSimple(ls *lua.LState, fnName, method string, withBody bool) int {
	rawURL := ls.CheckString(1)
	body := ""
	optsPos := 2
	if withBody {
		body = ls.OptString(2, "")
		optsPos = 3
	}
	headers, timeout, err := parseConvenienceOpts(ls, optsPos)
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, fnName, method, reqURL, headers, body, timeout)
}

// doHTTPRequest performs the actual HTTP request and pushes the result
// onto the Lua stack. Returns the number of values pushed (always 2).
// fnName is used as the prefix on raised errors so scripts see the
// entry-point name (e.g. "http.get") rather than always "http.request".
func (r *Runtime) doHTTPRequest(
	ls *lua.LState,
	fnName, method string,
	reqURL *url.URL,
	headers map[string]string,
	body string,
	timeout time.Duration,
) int {
	ctx := httpContext(r)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, reqURL.String(), bodyReader)
	if err != nil {
		ls.RaiseError("%s: %s", fnName, err.Error())
		return 0
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
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
// runtime's Lua-state context (for timeout) or falling back to Background.
func httpContext(r *Runtime) context.Context {
	if ctx := r.L.Context(); ctx != nil {
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
	timeout time.Duration
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
		tbl, ok := v.(*lua.LTable)
		if !ok {
			return out, errors.New("headers must be a table")
		}
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
			out.headers[string(ks)] = string(vs)
		})
		if headerErr != nil {
			return out, headerErr
		}
	}

	// body (optional)
	if v := opts.RawGetString("body"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return out, errors.New("body must be a string")
		}
		out.body = string(s)
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

// parseConvenienceOpts extracts headers and timeout from the optional
// opts table used by convenience methods (get, post, etc.). Returns an
// error rather than raising so the caller can prefix the function name
// (http.get / http.post / ...) — matches parseHTTPRequestOpts.
func parseConvenienceOpts(ls *lua.LState, pos int) (map[string]string, time.Duration, error) {
	headers := make(map[string]string)
	var timeout time.Duration

	optsTbl := ls.OptTable(pos, nil)
	if optsTbl == nil {
		return headers, timeout, nil
	}

	if v := optsTbl.RawGetString("headers"); v != lua.LNil {
		tbl, ok := v.(*lua.LTable)
		if !ok {
			return nil, 0, fmt.Errorf("headers must be a table, got %s", v.Type().String())
		}
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
			return nil, 0, headerErr
		}
	}

	if v := optsTbl.RawGetString("timeout"); v != lua.LNil {
		n, ok := v.(lua.LNumber)
		if !ok {
			return nil, 0, fmt.Errorf("timeout must be a number, got %s", v.Type().String())
		}
		if n <= 0 {
			return nil, 0, errors.New("timeout must be positive")
		}
		timeout = time.Duration(float64(n) * float64(time.Second))
	}

	return headers, timeout, nil
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
