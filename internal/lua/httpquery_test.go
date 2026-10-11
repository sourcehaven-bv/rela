package lua

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TKT-01KZSO: http.encode_query builds a sorted form-encoded string.
func TestLuaHTTP_EncodeQuery(t *testing.T) {
	t.Parallel()
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local q = http.encode_query({b = "x y", a = 1, c = true, d = {"1", 2}, e = "&="})
		assert(q == "a=1&b=x+y&c=true&d=1&d=2&e=%26%3D", "got " .. q)
		assert(http.encode_query({}) == "", "empty table encodes to empty string")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if err := rt.RunString(`http.encode_query({a = {{}}})`); err == nil {
		t.Fatal("a nested table value must raise")
	}
}

// TKT-01KZSO: a 429 or 503 carries the Retry-After wait; other statuses
// carry 0.
func TestLuaHTTP_RetryAfter(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/seconds":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/date":
			w.Header().Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/ok":
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	t.Cleanup(server.Close)

	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local base = "` + server.URL + `"
		local r = http.get(base .. "/seconds")
		assert(r.status_code == 429 and r.retry_after == 7, "seconds: " .. tostring(r.retry_after))
		r = http.get(base .. "/date")
		assert(r.retry_after >= 80 and r.retry_after <= 91, "date: " .. tostring(r.retry_after))
		r = http.get(base .. "/ok")
		assert(r.retry_after == 0, "200 carries no wait")
		r = http.get(base .. "/none")
		assert(r.retry_after == 0, "429 without header")
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

// TKT-01KZSO: a transport error never shows the query string, which often
// carries a credential.
func TestLuaHTTP_ErrorOmitsQuery(t *testing.T) {
	t.Parallel()
	rt := newHTTPRuntime(t)
	if err := rt.RunString(`
		local _, err = http.get("http://127.0.0.1:1/path?api_key=SECRET123#frag")
		assert(err.kind == "network", "kind = " .. tostring(err.kind))
		for _, text in ipairs({err.message, err.details}) do
			assert(not text:find("SECRET123", 1, true), "query leaked: " .. text)
			assert(not text:find("frag", 1, true), "fragment leaked: " .. text)
			assert(text:find("127.0.0.1:1/path", 1, true), "host and path missing: " .. text)
		end
	`); err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

func TestRedactURL(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"https://u:p@h.example/a?b=c#d": "https://h.example/a",
		"https://h.example/a?":          "https://h.example/a",
		"https://h.example/a":           "https://h.example/a",
		"%zz?secret":                    "%zz",
	}
	for in, want := range tests {
		if got := redactURL(in); got != want {
			t.Errorf("redactURL(%q) = %q, want %q", in, got, want)
		}
	}
}
