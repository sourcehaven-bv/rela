// Lua bindings for the http.* module.
//
// Provides HTTP client capabilities for Lua scripts to call external APIs.
// Follows the same error convention as the ai.* module:
//
//	expected runtime failure  -> (nil, err_table)
//	programming error         -> RaiseError
//
// The error table has stable fields: kind (string), status (number),
// message (string), details (string). Scripts branch on err.kind.
//
// Error kinds:
//   - timeout:      request exceeded deadline
//   - canceled:     request was canceled (e.g., runtime shutting down)
//   - network:      DNS, connection refused, TLS, etc.
//   - bad_response: response body too large or unreadable
//
// JSON helpers use the same convention split: json_encode raises on wrong
// arg types (programming error); json_decode returns (nil, err_table) for
// invalid JSON (expected runtime failure from external data).
package lua

import (
	"context"
	"encoding/json"
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
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: httpDefaultTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// httpClient is the shared HTTP client for all Lua HTTP requests within
// a process. Connection pooling is reused across requests.
var httpClient = newHTTPClient()

// registerHTTPModule installs the top-level `http` global with request,
// convenience, and JSON functions.
func (r *Runtime) registerHTTPModule() {
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "request", r.L.NewFunction(r.luaHTTPRequest))
	r.L.SetField(tbl, "get", r.L.NewFunction(r.luaHTTPGet))
	r.L.SetField(tbl, "post", r.L.NewFunction(r.luaHTTPPost))
	r.L.SetField(tbl, "put", r.L.NewFunction(r.luaHTTPPut))
	r.L.SetField(tbl, "patch", r.L.NewFunction(r.luaHTTPPatch))
	r.L.SetField(tbl, "delete", r.L.NewFunction(r.luaHTTPDelete))
	r.L.SetField(tbl, "json_encode", r.L.NewFunction(luaJSONEncode))
	r.L.SetField(tbl, "json_decode", r.L.NewFunction(luaJSONDecode))
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
	method, reqURL, headers, body, timeout, err := parseHTTPRequestOpts(opts)
	if err != nil {
		ls.RaiseError("http.request: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, method, reqURL, headers, body, timeout)
}

// luaHTTPGet implements http.get(url, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPGet(ls *lua.LState) int {
	rawURL := ls.CheckString(1)
	headers, timeout := parseConvenienceOpts(ls, 2)
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("http.get: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, "GET", reqURL, headers, "", timeout)
}

// luaHTTPPost implements http.post(url, body, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPPost(ls *lua.LState) int {
	rawURL := ls.CheckString(1)
	body := ls.OptString(2, "")
	headers, timeout := parseConvenienceOpts(ls, 3)
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("http.post: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, "POST", reqURL, headers, body, timeout)
}

// luaHTTPPut implements http.put(url, body, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPPut(ls *lua.LState) int {
	rawURL := ls.CheckString(1)
	body := ls.OptString(2, "")
	headers, timeout := parseConvenienceOpts(ls, 3)
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("http.put: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, "PUT", reqURL, headers, body, timeout)
}

// luaHTTPPatch implements http.patch(url, body, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPPatch(ls *lua.LState) int {
	rawURL := ls.CheckString(1)
	body := ls.OptString(2, "")
	headers, timeout := parseConvenienceOpts(ls, 3)
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("http.patch: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, "PATCH", reqURL, headers, body, timeout)
}

// luaHTTPDelete implements http.delete(url, opts?) -> (response, nil) | (nil, err).
func (r *Runtime) luaHTTPDelete(ls *lua.LState) int {
	rawURL := ls.CheckString(1)
	headers, timeout := parseConvenienceOpts(ls, 2)
	reqURL, err := validateURL(rawURL)
	if err != nil {
		ls.RaiseError("http.delete: %s", err.Error())
		return 0
	}
	return r.doHTTPRequest(ls, "DELETE", reqURL, headers, "", timeout)
}

// doHTTPRequest performs the actual HTTP request and pushes the result
// onto the Lua stack. Returns the number of values pushed (always 2).
func (r *Runtime) doHTTPRequest(
	ls *lua.LState,
	method string,
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
		// Should not happen — URL and method are pre-validated.
		ls.RaiseError("http.request: %s", err.Error())
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

// parseHTTPRequestOpts extracts fields from the opts table for http.request().
func parseHTTPRequestOpts(opts *lua.LTable) (string, *url.URL, map[string]string, string, time.Duration, error) {
	// url (required)
	urlVal := opts.RawGetString("url")
	urlStr, ok := urlVal.(lua.LString)
	if !ok || urlStr == "" {
		return "", nil, nil, "", 0, errors.New("url must be a non-empty string")
	}

	reqURL, err := validateURL(string(urlStr))
	if err != nil {
		return "", nil, nil, "", 0, err
	}

	// method (optional, default GET)
	method := "GET"
	if v := opts.RawGetString("method"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return "", nil, nil, "", 0, errors.New("method must be a string")
		}
		method = strings.ToUpper(string(s))
	}

	// headers (optional)
	headers := make(map[string]string)
	if v := opts.RawGetString("headers"); v != lua.LNil {
		tbl, ok := v.(*lua.LTable)
		if !ok {
			return "", nil, nil, "", 0, errors.New("headers must be a table")
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
			return "", nil, nil, "", 0, headerErr
		}
	}

	// body (optional)
	var body string
	if v := opts.RawGetString("body"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return "", nil, nil, "", 0, errors.New("body must be a string")
		}
		body = string(s)
	}

	// timeout (optional, seconds)
	var timeout time.Duration
	if v := opts.RawGetString("timeout"); v != lua.LNil {
		n, ok := v.(lua.LNumber)
		if !ok {
			return "", nil, nil, "", 0, errors.New("timeout must be a number")
		}
		if n <= 0 {
			return "", nil, nil, "", 0, errors.New("timeout must be positive")
		}
		timeout = time.Duration(float64(n) * float64(time.Second))
	}

	return method, reqURL, headers, body, timeout, nil
}

// parseConvenienceOpts extracts headers and timeout from the optional
// opts table used by convenience methods (get, post, etc.).
// Raises on type mismatches (consistent with parseHTTPRequestOpts).
func parseConvenienceOpts(ls *lua.LState, pos int) (map[string]string, time.Duration) {
	headers := make(map[string]string)
	var timeout time.Duration

	optsTbl := ls.OptTable(pos, nil)
	if optsTbl == nil {
		return headers, timeout
	}

	if v := optsTbl.RawGetString("headers"); v != lua.LNil {
		tbl, ok := v.(*lua.LTable)
		if !ok {
			ls.RaiseError("headers must be a table, got %s", v.Type().String())
			return headers, timeout
		}
		tbl.ForEach(func(k, v lua.LValue) {
			ks, kok := k.(lua.LString)
			if !kok {
				ls.RaiseError("header key must be a string, got %s", k.Type().String())
				return
			}
			vs, vok := v.(lua.LString)
			if !vok {
				ls.RaiseError("header value for %q must be a string, got %s", string(ks), v.Type().String())
				return
			}
			headers[string(ks)] = string(vs)
		})
	}

	if v := optsTbl.RawGetString("timeout"); v != lua.LNil {
		n, ok := v.(lua.LNumber)
		if !ok {
			ls.RaiseError("timeout must be a number, got %s", v.Type().String())
			return headers, timeout
		}
		if n > 0 {
			timeout = time.Duration(float64(n) * float64(time.Second))
		}
	}

	return headers, timeout
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
	return u, nil
}

// httpError represents an HTTP-level error surfaced to Lua scripts.
type httpError struct {
	Kind    string
	Status  int
	Message string
	Details string
}

// classifyHTTPError converts a net/http client error into an httpError.
func classifyHTTPError(err error) *httpError {
	if err == nil {
		return nil
	}
	msg := err.Error()

	if errors.Is(err, context.DeadlineExceeded) {
		return &httpError{Kind: "timeout", Message: msg, Details: msg}
	}
	if errors.Is(err, context.Canceled) {
		return &httpError{Kind: "canceled", Message: msg, Details: msg}
	}

	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return &httpError{Kind: "timeout", Message: msg, Details: msg}
	}

	return &httpError{Kind: "network", Message: msg, Details: msg}
}

// errHTTPBodyTooLarge is returned when the response exceeds httpMaxResponseBytes.
var errHTTPBodyTooLarge = errors.New("response body exceeded 10 MiB limit")

// readHTTPBody reads up to httpMaxResponseBytes from the response body.
func readHTTPBody(r io.Reader) ([]byte, *httpError) {
	limited := io.LimitReader(r, httpMaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, &httpError{
			Kind:    "network",
			Message: "reading response body: " + err.Error(),
			Details: err.Error(),
		}
	}
	if int64(len(body)) > httpMaxResponseBytes {
		return nil, &httpError{
			Kind:    "bad_response",
			Message: errHTTPBodyTooLarge.Error(),
			Details: errHTTPBodyTooLarge.Error(),
		}
	}
	return body, nil
}

// pushHTTPError pushes (nil, err_table) onto the Lua stack.
func pushHTTPError(ls *lua.LState, e *httpError) int {
	ls.Push(lua.LNil)
	tbl := ls.NewTable()
	tbl.RawSetString("kind", lua.LString(e.Kind))
	tbl.RawSetString("status", lua.LNumber(e.Status))
	tbl.RawSetString("message", lua.LString(e.Message))
	tbl.RawSetString("details", lua.LString(e.Details))
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

// luaJSONEncode implements http.json_encode(value) -> string.
// Raises on wrong arg type or encoding failure.
func luaJSONEncode(ls *lua.LState) int {
	val := ls.CheckAny(1)
	goVal := luaValueToGo(val)
	data, err := json.Marshal(goVal)
	if err != nil {
		ls.RaiseError("http.json_encode: %s", err.Error())
		return 0
	}
	ls.Push(lua.LString(string(data)))
	return 1
}

// luaJSONDecode implements http.json_decode(string) -> (value, nil) | (nil, err_table).
// Wrong arg type raises; invalid JSON returns (nil, err_table).
func luaJSONDecode(ls *lua.LState) int {
	str := ls.CheckString(1)
	var goVal interface{}
	if err := json.Unmarshal([]byte(str), &goVal); err != nil {
		return pushHTTPError(ls, &httpError{
			Kind:    "bad_response",
			Message: "json_decode: " + err.Error(),
			Details: err.Error(),
		})
	}
	ls.Push(goValueToLua(ls, goVal))
	ls.Push(lua.LNil)
	return 2
}

// goValueToLua converts a Go value (from json.Unmarshal) to a Lua value.
func goValueToLua(ls *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := ls.NewTable()
		for i, item := range v {
			tbl.RawSetInt(i+1, goValueToLua(ls, item))
		}
		return tbl
	case map[string]interface{}:
		tbl := ls.NewTable()
		for key, item := range v {
			tbl.RawSetString(key, goValueToLua(ls, item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}
