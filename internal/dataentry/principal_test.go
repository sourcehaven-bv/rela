package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestStampAuditPrincipal_DefaultResolver verifies that every request
// flowing through the middleware with the default resolver carries
// Principal{User:"unknown", Tool:"data-entry"} (per design-review:
// the server's $USER would be misleading for human edits;
// per-request override lands in a follow-up).
//
// Satisfies AC4 for the data-entry entry point.
func TestStampAuditPrincipal_DefaultResolver(t *testing.T) {
	var captured principal.Principal
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = principal.From(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := stampAuditPrincipal(captureHandler, defaultPrincipalResolver)

	req := httptest.NewRequest(http.MethodGet, "/anything", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", captured.Tool, principal.ToolDataEntry)
	}
	if captured.User != "unknown" {
		t.Errorf("User = %q, want 'unknown' (per design-review: per-request override is a follow-up)",
			captured.User)
	}
}

// TestStampAuditPrincipal_CustomResolver verifies the seam works for
// the follow-up PR: a header-aware resolver returns a per-request
// Principal that the middleware stamps on the ctx.
func TestStampAuditPrincipal_CustomResolver(t *testing.T) {
	resolver := func(r *http.Request) principal.Principal {
		return principal.Principal{
			User: r.Header.Get("X-Test-User"),
			Tool: principal.ToolDataEntry,
		}
	}

	var captured principal.Principal
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = principal.From(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := stampAuditPrincipal(captureHandler, resolver)

	req := httptest.NewRequest(http.MethodGet, "/anything", http.NoBody)
	req.Header.Set("X-Test-User", "alice")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured.User != "alice" {
		t.Errorf("User = %q, want 'alice' (resolver should read header)", captured.User)
	}
	if captured.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", captured.Tool, principal.ToolDataEntry)
	}
}

// --- TKT-WEBI: per-request Principal from HTTP header ---

// runResolver applies resolver to a request crafted from headers and
// returns the captured Principal. Shared plumbing for AC1-AC7.
func runResolver(t *testing.T, resolver PrincipalResolver, headers map[string]string) principal.Principal {
	t.Helper()
	var captured principal.Principal
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = principal.From(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := stampAuditPrincipal(captureHandler, resolver)

	req := httptest.NewRequest(http.MethodGet, "/anything", http.NoBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return captured
}

// AC1: header populates Principal.User when configured.
func TestHeaderPrincipalResolver_PopulatesUser(t *testing.T) {
	got := runResolver(t,
		ChainResolvers(HeaderPrincipalResolver("X-User")),
		map[string]string{"X-User": "alice"})
	if got.User != "alice" {
		t.Errorf("User = %q, want 'alice'", got.User)
	}
	if got.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", got.Tool, principal.ToolDataEntry)
	}
}

// AC2: missing header falls through to "unknown"; no 401.
func TestHeaderPrincipalResolver_AbsentHeaderFallsThrough(t *testing.T) {
	got := runResolver(t,
		ChainResolvers(HeaderPrincipalResolver("X-User")),
		nil)
	if got.User != "unknown" {
		t.Errorf("User = %q, want 'unknown'", got.User)
	}
	if got.Tool != principal.ToolDataEntry {
		t.Errorf("Tool = %q, want %q", got.Tool, principal.ToolDataEntry)
	}
}

// AC3: empty / whitespace-only header value falls through to "unknown".
func TestHeaderPrincipalResolver_EmptyHeaderFallsThrough(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"tab+space", "\t  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runResolver(t,
				ChainResolvers(HeaderPrincipalResolver("X-User")),
				map[string]string{"X-User": tt.value})
			if got.User != "unknown" {
				t.Errorf("User = %q, want 'unknown'", got.User)
			}
		})
	}
}

// resolveHeaderRaw builds a request that bypasses http.Header.Set's
// CR/LF rejection by writing the canonical-form map directly. Used
// for sanitization tests whose inputs would otherwise be silently
// dropped by net/http.
func resolveHeaderRaw(t *testing.T, headerName, value string) principal.Principal {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header[headerName] = []string{value}
	return HeaderPrincipalResolver(headerName)(req)
}

// AC4: header value is sanitized.
func TestHeaderPrincipalResolver_Sanitizes(t *testing.T) {
	t.Run("control chars replaced", func(t *testing.T) {
		p := resolveHeaderRaw(t, "X-User", "alice\nbob")
		if p.User != "alice bob" {
			t.Errorf("User = %q, want 'alice bob' (newline replaced with space)", p.User)
		}
	})
	t.Run("null byte replaced", func(t *testing.T) {
		p := resolveHeaderRaw(t, "X-User", "ali\x00ce")
		if p.User != "ali ce" {
			t.Errorf("User = %q, want 'ali ce'", p.User)
		}
	})
	t.Run("control-only payload sanitizes to empty (no spoof via NULs)", func(t *testing.T) {
		// Regression for the cranky review: a header value of pure NULs
		// must NOT survive sanitization as literal spaces. TrimSpace
		// after replacement catches it.
		p := resolveHeaderRaw(t, "X-User", "\x00\x00\x00")
		if p.User != "" {
			t.Errorf("User = %q, want '' (control-only payload must sanitize to empty)", p.User)
		}
	})
	t.Run("truncated at 256 runes", func(t *testing.T) {
		long := strings.Repeat("a", 1000)
		got := runResolver(t,
			ChainResolvers(HeaderPrincipalResolver("X-User")),
			map[string]string{"X-User": long})
		if runeLen := len([]rune(got.User)); runeLen != 256 {
			t.Errorf("rune count = %d, want 256", runeLen)
		}
	})
	t.Run("multi-byte runes preserved", func(t *testing.T) {
		got := runResolver(t,
			ChainResolvers(HeaderPrincipalResolver("X-User")),
			map[string]string{"X-User": "アリス"})
		if got.User != "アリス" {
			t.Errorf("User = %q, want 'アリス'", got.User)
		}
	})
}

// AC5: $RELA_DATAENTRY_USER env override wins over the header.
func TestChainResolvers_EnvWinsOverHeader(t *testing.T) {
	chain := ChainResolvers(EnvPrincipalResolver(), HeaderPrincipalResolver("X-User"))

	t.Run("env set, header set: env wins", func(t *testing.T) {
		t.Setenv(envDataEntryUser, "operator")
		got := runResolver(t, chain, map[string]string{"X-User": "alice"})
		if got.User != "operator" {
			t.Errorf("User = %q, want 'operator' (env wins over header)", got.User)
		}
	})
	t.Run("env unset, header set: header wins", func(t *testing.T) {
		t.Setenv(envDataEntryUser, "")
		got := runResolver(t, chain, map[string]string{"X-User": "alice"})
		if got.User != "alice" {
			t.Errorf("User = %q, want 'alice' (env empty, header alone)", got.User)
		}
	})
	t.Run("env whitespace, header set: header wins", func(t *testing.T) {
		t.Setenv(envDataEntryUser, "   ")
		got := runResolver(t, chain, map[string]string{"X-User": "alice"})
		if got.User != "alice" {
			t.Errorf("User = %q, want 'alice' (env whitespace treated as empty)", got.User)
		}
	})
	t.Run("both unset: falls through to unknown", func(t *testing.T) {
		t.Setenv(envDataEntryUser, "")
		got := runResolver(t, chain, nil)
		if got.User != "unknown" {
			t.Errorf("User = %q, want 'unknown'", got.User)
		}
	})
}

// AC6: empty headerName (flag unset) returns zero principal — chain
// falls through to default. Production: rela-server without
// --principal-header behaves exactly as before this PR.
func TestHeaderPrincipalResolver_EmptyNameDisabled(t *testing.T) {
	got := runResolver(t,
		ChainResolvers(HeaderPrincipalResolver("")),
		map[string]string{"X-Anything": "alice"})
	if got.User != "unknown" {
		t.Errorf("User = %q, want 'unknown' (empty header name disables resolver)", got.User)
	}
}

// AC7: Tool is always ToolDataEntry, regardless of resolver source.
func TestHeaderPrincipalResolver_ToolUnchanged(t *testing.T) {
	tests := []struct {
		name     string
		resolver PrincipalResolver
		envValue string
		headers  map[string]string
	}{
		{"header", HeaderPrincipalResolver("X-User"), "", map[string]string{"X-User": "alice"}},
		{"env", EnvPrincipalResolver(), "operator", nil},
		{"default", defaultPrincipalResolver, "", nil},
		// chain-fallback: nothing resolves, ChainResolvers falls back
		// to defaultPrincipalResolver — verify the fallback path
		// itself returns Tool=ToolDataEntry, not just the bare default.
		{"chain-fallback", ChainResolvers(HeaderPrincipalResolver("X-User")), "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(envDataEntryUser, tt.envValue)
			}
			got := runResolver(t, tt.resolver, tt.headers)
			if got.Tool != principal.ToolDataEntry {
				t.Errorf("Tool = %q, want %q", got.Tool, principal.ToolDataEntry)
			}
		})
	}
}

// Negative: a syntactically invalid header name doesn't panic; just
// no match (Go's http.Header normalizes / canonicalizes names, so a
// bad name lookups to an empty value).
func TestHeaderPrincipalResolver_WeirdHeaderName(t *testing.T) {
	got := runResolver(t,
		ChainResolvers(HeaderPrincipalResolver("X-Weird Name")),
		map[string]string{"X-User": "alice"})
	if got.User != "unknown" {
		t.Errorf("User = %q, want 'unknown' (weird header name → no match)", got.User)
	}
}
