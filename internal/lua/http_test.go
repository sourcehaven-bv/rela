package lua

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newHTTPRuntime builds a Runtime for HTTP module testing.
func newHTTPRuntime(t *testing.T) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	rt := New(ws, ws.Meta(), t.TempDir(), &buf)
	t.Cleanup(rt.Close)
	return rt
}

func TestLuaHTTP_GlobalRegistered(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		assert(type(http) == "table", "http global must be a table")
		assert(type(http.request) == "function", "http.request must be a function")
		assert(type(http.get) == "function", "http.get must be a function")
		assert(type(http.post) == "function", "http.post must be a function")
		assert(type(http.put) == "function", "http.put must be a function")
		assert(type(http.patch) == "function", "http.patch must be a function")
		assert(type(http.delete) == "function", "http.delete must be a function")
		assert(type(http.json_encode) == "function", "http.json_encode must be a function")
		assert(type(http.json_decode) == "function", "http.json_decode must be a function")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_RequestGetSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.request({url = "` + server.URL + `"})
		assert(err == nil, "expected nil err, got " .. tostring(err))
		assert(type(resp) == "table", "resp should be a table")
		assert(resp.status_code == 200, "status_code = " .. tostring(resp.status_code))
		assert(resp.body == '{"result":"ok"}', "body = " .. tostring(resp.body))
		assert(type(resp.headers) == "table", "headers should be a table")
		assert(resp.headers["content-type"] == "application/json", "content-type = " .. tostring(resp.headers["content-type"]))
		assert(resp.headers["x-custom"] == "test-value", "x-custom = " .. tostring(resp.headers["x-custom"]))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_RequestPostWithBody(t *testing.T) {
	var receivedBody string
	var receivedMethod string
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.request({
			url = "` + server.URL + `",
			method = "post",
			headers = {["Content-Type"] = "application/json"},
			body = '{"name":"test"}',
		})
		assert(err == nil, "expected nil err")
		assert(resp.status_code == 201, "status_code = " .. tostring(resp.status_code))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("expected POST (uppercased), got %s", receivedMethod)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected application/json, got %s", receivedContentType)
	}
	if receivedBody != `{"name":"test"}` {
		t.Errorf("body = %s", receivedBody)
	}
}

func TestLuaHTTP_ConvenienceGet(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `")
		assert(err == nil)
		assert(resp.status_code == 200)
		assert(resp.body == "ok")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedMethod != "GET" {
		t.Errorf("expected GET, got %s", receivedMethod)
	}
}

func TestLuaHTTP_ConveniencePost(t *testing.T) {
	var receivedMethod string
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.post("` + server.URL + `", '{"x":1}')
		assert(err == nil)
		assert(resp.status_code == 200)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedBody != `{"x":1}` {
		t.Errorf("body = %s", receivedBody)
	}
}

func TestLuaHTTP_ConveniencePut(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.put("` + server.URL + `", "data")
		assert(err == nil)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
}

func TestLuaHTTP_ConveniencePatch(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.patch("` + server.URL + `", "data")
		assert(err == nil)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", receivedMethod)
	}
}

func TestLuaHTTP_ConvenienceDelete(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.delete("` + server.URL + `")
		assert(err == nil)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", receivedMethod)
	}
}

func TestLuaHTTP_ConvenienceWithOpts(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `", {
			headers = {Authorization = "Bearer test-token"},
		})
		assert(err == nil)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Bearer test-token, got %q", receivedAuth)
	}
}

func TestLuaHTTP_TimeoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte("too late"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.request({
			url = "` + server.URL + `",
			timeout = 0.1,
		})
		assert(resp == nil, "resp should be nil on timeout")
		assert(type(err) == "table", "err should be a table")
		assert(err.kind == "timeout", "kind = " .. tostring(err.kind))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

// TestClassifyHTTPError_Canceled verifies that context.Canceled (the error
// surfaced when the runtime's parent context is canceled mid-request) is
// classified as kind="canceled" rather than "timeout" or "network".
//
// This is a unit test of the classifier rather than an end-to-end
// RunString test because the Lua runtime itself short-circuits with a
// Lua-level "context canceled" error on the next opcode after the parent
// context is canceled, which prevents a script from observing the
// err_table. The real shutdown path still produces a canceled err_table
// via this classifier; scripts that want to observe it would need to
// return before the next opcode (not a realistic scenario).
func TestClassifyHTTPError_Canceled(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"direct context.Canceled", context.Canceled, "canceled"},
		{"wrapped context.Canceled", &url.Error{Op: "Get", URL: "http://x", Err: context.Canceled}, "canceled"},
		{"direct context.DeadlineExceeded", context.DeadlineExceeded, "timeout"},
		{"wrapped deadline exceeded", &url.Error{Op: "Get", URL: "http://x", Err: context.DeadlineExceeded}, "timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyHTTPError(tc.err)
			if got == nil {
				t.Fatal("classifyHTTPError returned nil")
			}
			if got.Kind != tc.want {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.want)
			}
		})
	}
}

func TestLuaHTTP_NetworkError(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("http://127.0.0.1:1")
		assert(resp == nil, "resp should be nil on network error")
		assert(type(err) == "table", "err should be a table")
		assert(err.kind == "network", "kind = " .. tostring(err.kind))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_ResponseBodyTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Write more than 10 MiB
		data := make([]byte, 11*1024*1024)
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `")
		assert(resp == nil, "resp should be nil when body too large")
		assert(err.kind == "bad_response", "kind = " .. tostring(err.kind))
		assert(string.find(err.message, "10 MiB"), "message should mention limit: " .. err.message)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_EmptyResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `")
		assert(err == nil)
		assert(resp.status_code == 204)
		assert(resp.body == "", "body should be empty, got: " .. resp.body)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_RedirectNotFollowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://example.com/other")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `")
		assert(err == nil)
		assert(resp.status_code == 302, "status_code = " .. tostring(resp.status_code))
		assert(resp.headers["location"] == "http://example.com/other")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_NonSuccessStatusReturnsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `")
		assert(err == nil, "non-2xx should still return response, not error")
		assert(resp.status_code == 404)
		assert(resp.body == "not found")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

// --- Programming error tests (RaiseError) ---

func TestLuaHTTP_RequestMissingURL(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.request({})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
	if !strings.Contains(err.Error(), "url must be a non-empty string") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaHTTP_RequestInvalidURL(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.request({url = "not-a-url"})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
	if !strings.Contains(err.Error(), "http or https scheme") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaHTTP_RequestFTPScheme(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.request({url = "ftp://example.com/file"})`)
	if err == nil {
		t.Fatal("expected Lua error")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaHTTP_RequestNoArgs(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.request()`)
	if err == nil {
		t.Fatal("expected Lua error for missing argument")
	}
}

func TestLuaHTTP_GetNoArgs(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.get()`)
	if err == nil {
		t.Fatal("expected Lua error for missing argument")
	}
}

// --- JSON tests ---

func TestLuaHTTP_JSONEncodeTable(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local s = http.json_encode({name = "test", value = 42})
		assert(type(s) == "string", "should return string")
		-- Decode to verify (round-trip)
		local t, err = http.json_decode(s)
		assert(err == nil)
		assert(t.name == "test")
		assert(t.value == 42)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONEncodeArray(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local s = http.json_encode({"a", "b", "c"})
		assert(s == '["a","b","c"]', "got: " .. s)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONEncodeNested(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local s = http.json_encode({items = {"x", "y"}, count = 2})
		local t, err = http.json_decode(s)
		assert(err == nil)
		assert(t.count == 2)
		assert(t.items[1] == "x")
		assert(t.items[2] == "y")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONEncodePrimitives(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		assert(http.json_encode("hello") == '"hello"')
		assert(http.json_encode(42) == '42')
		assert(http.json_encode(true) == 'true')
		assert(http.json_encode(false) == 'false')
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONDecodeObject(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local t, err = http.json_decode('{"name":"test","value":42,"active":true}')
		assert(err == nil)
		assert(t.name == "test")
		assert(t.value == 42)
		assert(t.active == true)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONDecodeArray(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local t, err = http.json_decode('[1, 2, 3]')
		assert(err == nil)
		assert(t[1] == 1)
		assert(t[2] == 2)
		assert(t[3] == 3)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONDecodeNested(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local t, err = http.json_decode('{"items":[{"id":1},{"id":2}]}')
		assert(err == nil)
		assert(t.items[1].id == 1)
		assert(t.items[2].id == 2)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONDecodeNull(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local t, err = http.json_decode('{"key":null}')
		assert(err == nil)
		assert(t.key == nil)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONDecodeInvalidReturnsError(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local result, err = http.json_decode("not valid json")
		assert(result == nil, "result should be nil for invalid JSON")
		assert(type(err) == "table", "err should be a table")
		assert(err.kind == "bad_response", "kind = " .. tostring(err.kind))
		assert(string.find(err.message, "json_decode"), "message should mention json_decode")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

// TestLuaHTTP_JSONEncodeEmptyTableIsObject codifies a known limitation: an
// empty Lua table {} encodes as a JSON object {}, not an array []. Lua has no
// way to mark "this empty table is intended as an array." Scripts needing an
// empty array should construct it server-side or send a non-empty placeholder.
func TestLuaHTTP_JSONEncodeEmptyTableIsObject(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local s, err = http.json_encode({})
		assert(err == nil, "err = " .. tostring(err))
		assert(s == "{}", "expected empty object, got " .. tostring(s))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONEncodeCycleSubstitutesSentinel(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local t = {}
		t.self = t
		local s, err = http.json_encode(t)
		assert(err == nil, "err should be nil, got " .. tostring(err))
		-- json.Marshal escapes < as <, so check for the escaped form too.
		assert(string.find(s, "cycle", 1, true), "encoded JSON should contain cycle sentinel, got " .. s)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONEncodeDeepTableSubstitutesSentinel(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		-- Build a table 100 levels deep (cap is 64).
		local t = {}
		local cur = t
		for i = 1, 100 do
			cur.next = {}
			cur = cur.next
		end
		local s, err = http.json_encode(t)
		assert(err == nil, "err should be nil, got " .. tostring(err))
		assert(string.find(s, "max-depth", 1, true), "encoded JSON should contain max-depth sentinel, got " .. s)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONDecodeDeepResponseSubstitutesSentinel(t *testing.T) {
	rt := newHTTPRuntime(t)
	// Build a deeply-nested JSON string: {"n":{"n":{"n":...}}} 100 levels deep.
	deep := strings.Repeat(`{"n":`, 100) + `null` + strings.Repeat(`}`, 100)
	if err := rt.RunString(`
		local t, err = http.json_decode([[` + deep + `]])
		assert(err == nil, "err should be nil, got " .. tostring(err))
		-- Walk down until we hit the sentinel string instead of a table.
		local cur = t
		local depth = 0
		while type(cur) == "table" do
			cur = cur.n
			depth = depth + 1
			if depth > 200 then error("exceeded walk limit") end
		end
		assert(cur == "<max-depth>", "expected sentinel at max depth, got " .. tostring(cur))
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_JSONEncodeNoArgs(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.json_encode()`)
	if err == nil {
		t.Fatal("expected Lua error for missing argument")
	}
}

func TestLuaHTTP_JSONDecodeWrongType(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.json_decode({})`)
	if err == nil {
		t.Fatal("expected Lua error for wrong type")
	}
	if !strings.Contains(err.Error(), "string expected") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaHTTP_NonStringHeaderValueRaises(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`
		http.request({
			url = "http://example.com",
			headers = {Authorization = 123},
		})
	`)
	if err == nil {
		t.Fatal("expected Lua error for non-string header value")
	}
	if !strings.Contains(err.Error(), "header value") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaHTTP_NonStringHeaderValueInConvenienceRaises(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`
		http.get("http://example.com", {
			headers = {["X-Version"] = 42},
		})
	`)
	if err == nil {
		t.Fatal("expected Lua error for non-string header value in convenience method")
	}
	if !strings.Contains(err.Error(), "header value") {
		t.Errorf("error = %v", err)
	}
}

func TestLuaHTTP_DefaultTimeoutApplied(t *testing.T) {
	// Verify that requests without explicit timeout still have the 30s default.
	// We can't easily test this directly, but we can verify the server receives
	// the request (proving the shared client works).
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received = true
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("` + server.URL + `")
		assert(err == nil)
		assert(resp.status_code == 200)
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if !received {
		t.Error("server did not receive request")
	}
}

// --- Error table shape test ---

func TestLuaHTTP_ErrorTableShape(t *testing.T) {
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local resp, err = http.get("http://127.0.0.1:1")
		assert(resp == nil)
		assert(type(err) == "table")
		assert(type(err.kind) == "string", "kind should be string")
		assert(type(err.status) == "number", "status should be number")
		assert(type(err.message) == "string", "message should be string")
		assert(type(err.retry_after) == "number", "retry_after should be number")
		assert(type(err.details) == "string", "details should be string")
		-- details exposes the unwrapped cause, so it should be non-empty for
		-- network errors (where an underlying net.Error is wrapped).
		assert(#err.details > 0, "details should be non-empty for network error")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestLuaHTTP_InvalidMethodRaises(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.request({url = "http://127.0.0.1:1", method = "GET HACK"})`)
	if err == nil {
		t.Fatal("expected Lua error for method with whitespace")
	}
}

func TestLuaHTTP_ConvenienceTimeoutZeroRaises(t *testing.T) {
	rt := newHTTPRuntime(t)
	err := rt.RunString(`http.get("http://127.0.0.1:1", {timeout = 0})`)
	if err == nil {
		t.Fatal("expected Lua error for timeout=0 on convenience method")
	}
}

// --- Integration-style test: full API call flow ---

func TestLuaHTTP_FullAPIFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"query"`) {
			t.Errorf("body should contain query: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":1,"name":"first"},{"id":2,"name":"second"}]}}`))
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		-- Build request
		local payload = http.json_encode({query = "SELECT * FROM items"})

		-- Make request
		local resp, err = http.post("` + server.URL + `/api", payload, {
			headers = {["Content-Type"] = "application/json"},
		})
		assert(err == nil, "request failed: " .. tostring(err))
		assert(resp.status_code == 200)

		-- Parse response
		local data, jerr = http.json_decode(resp.body)
		assert(jerr == nil, "decode failed")
		assert(data.data.items[1].id == 1)
		assert(data.data.items[1].name == "first")
		assert(data.data.items[2].name == "second")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}
