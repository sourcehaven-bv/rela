package script

import (
	"strconv"
	"strings"
	"testing"
)

// TestParseActionResponse_SPAShapeUnchanged is the compatibility pin for
// TKT-EFMRQM: every action that returns today's shape must produce exactly the
// same ActionResponse it produced before, with the rich fields left zero.
func TestParseActionResponse_SPAShapeUnchanged(t *testing.T) {
	resp, err := parseActionResponse(map[string]any{
		"redirect":     "/tickets",
		"message":      "saved",
		"message_type": "success",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Redirect != "/tickets" || resp.Message != "saved" || resp.MessageType != "success" {
		t.Fatalf("SPA fields not preserved: %+v", resp)
	}
	if resp.Status != 0 || resp.Body != "" || resp.ContentType != "" {
		t.Fatalf("rich fields must stay zero for an SPA-shaped return: %+v", resp)
	}
	if resp.IsRich() {
		t.Fatal("an SPA-shaped return must not be rich")
	}
}

// TestParseActionResponse_Rich covers the opt-in status/body/content_type form.
func TestParseActionResponse_Rich(t *testing.T) {
	resp, err := parseActionResponse(map[string]any{
		"status":       float64(202),
		"body":         `{"ok":true}`,
		"content_type": "application/json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsRich() {
		t.Fatal("expected a rich response")
	}
	if resp.Status != 202 || resp.Body != `{"ok":true}` || resp.ContentType != "application/json" {
		t.Fatalf("rich fields not parsed: %+v", resp)
	}
}

// TestParseActionResponse_RichDefaults pins the defaults: a body with no
// status is 200, and a body with no content_type is text/plain (never sniffed).
func TestParseActionResponse_RichDefaults(t *testing.T) {
	resp, err := parseActionResponse(map[string]any{"body": "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if resp.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("content_type = %q, want text/plain; charset=utf-8", resp.ContentType)
	}
}

// TestParseActionResponse_MixedShapeRefused refuses a return that sets both
// vocabularies. The two are answered to different consumers — the SPA reads
// {redirect,message}, a third party reads the body — and silently honoring one
// while dropping the other is the quiet failure this refuses to ship.
func TestParseActionResponse_MixedShapeRefused(t *testing.T) {
	_, err := parseActionResponse(map[string]any{
		"message": "saved",
		"status":  float64(200),
	})
	if err == nil {
		t.Fatal("expected a mixed-shape return to be refused")
	}
	if !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestParseActionResponse_ContentTypeAllowlist pins that a script may only
// choose from a small set of machine-readable types.
func TestParseActionResponse_ContentTypeAllowlist(t *testing.T) {
	tests := []struct {
		contentType string
		wantErr     bool
	}{
		{"application/json", false},
		{"application/json; charset=utf-8", false},
		{"text/plain", false},
		{"text/csv", false},
		{"application/xml", false},
		{"text/xml", false},
		{"APPLICATION/JSON", false},
		{"text/html", true},
		{"image/svg+xml", true},
		{"application/xhtml+xml", true},
		{"text/plain\r\nX-Injected: 1", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.contentType, func(t *testing.T) {
			_, err := parseActionResponse(map[string]any{
				"body":         "x",
				"content_type": tc.contentType,
			})
			if tc.wantErr && err == nil {
				t.Fatalf("expected %q to be refused", tc.contentType)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected %q to be accepted, got: %v", tc.contentType, err)
			}
		})
	}
}

// TestParseActionResponse_StatusRange refuses statuses a script has no business
// choosing. 1xx and 3xx are protocol-level: a script-chosen 101 or 302 would
// have the net/http machinery interpret a body that is not one, and 3xx is what
// `redirect:` is for (which is validated against open-redirect separately).
func TestParseActionResponse_StatusRange(t *testing.T) {
	tests := []struct {
		status  int
		wantErr bool
	}{
		{200, false},
		{201, false},
		{202, false},
		{204, false},
		{400, false},
		{409, false},
		{422, false},
		{500, false},
		{503, false},
		{100, true},
		{302, true},
		{600, true},
		{99, true},
		{-1, true},
	}
	for _, tc := range tests {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			// Both numeric shapes: int64 is what a real script produces,
			// float64 is what a fractional literal produces.
			for _, status := range []any{int64(tc.status), float64(tc.status)} {
				_, err := parseActionResponse(map[string]any{"status": status})
				if tc.wantErr && err == nil {
					t.Fatalf("expected status %d (%T) to be refused", tc.status, status)
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("expected status %d (%T) to be accepted, got: %v", tc.status, status, err)
				}
			}
		})
	}
}

// TestParseActionResponse_StatusFromInt64 pins the type a status ACTUALLY
// arrives as. lua.luaValueToGo narrows an integral Lua number to int64, so
// `status = 202` reaches here as int64 and not float64 — a parser that accepted
// only float64 passed every hand-built-map unit test and failed on every real
// script (caught end to end, not here, which is why this case now exists).
func TestParseActionResponse_StatusFromInt64(t *testing.T) {
	resp, err := parseActionResponse(map[string]any{
		"status": int64(202),
		"body":   "ok",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != 202 {
		t.Fatalf("status = %d, want 202", resp.Status)
	}
}

// TestParseActionResponse_NonIntegerStatus refuses a fractional status: Lua has
// one number type, so 200.5 arrives as a float64 and truncating it would let a
// typo become a silently different status.
func TestParseActionResponse_NonIntegerStatus(t *testing.T) {
	if _, err := parseActionResponse(map[string]any{"status": 200.5}); err == nil {
		t.Fatal("expected a fractional status to be refused")
	}
}

// TestParseActionResponse_StatusRejectsNonNumbers covers the remaining ways a
// status can arrive wrong, so no branch of actionStatusFrom is unreachable
// code. A huge int64 is representable in Lua and would otherwise be truncated
// into a valid-looking status by the int conversion.
func TestParseActionResponse_StatusRejectsNonNumbers(t *testing.T) {
	tests := []struct {
		name   string
		status any
	}{
		{"string", "200"},
		{"bool", true},
		{"table", map[string]any{"code": int64(200)}},
		{"nil", nil},
		{"int64 past int32", int64(1) << 40},
		{"negative int64 past int32", -(int64(1) << 40)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseActionResponse(map[string]any{"status": tc.status}); err == nil {
				t.Fatalf("expected status %#v to be refused", tc.status)
			}
		})
	}
}

// TestParseActionResponse_ContentTypeMustBeString covers the non-string
// content_type branch.
func TestParseActionResponse_ContentTypeMustBeString(t *testing.T) {
	if _, err := parseActionResponse(map[string]any{
		"body":         "x",
		"content_type": int64(1),
	}); err == nil {
		t.Fatal("expected a non-string content_type to be refused")
	}
}

// TestParseActionResponse_BodyMustBeString refuses a table body. Serializing it
// here would silently choose an encoding the script did not ask for; a script
// that wants JSON calls json.encode and sets content_type itself.
func TestParseActionResponse_BodyMustBeString(t *testing.T) {
	_, err := parseActionResponse(map[string]any{"body": map[string]any{"a": "b"}})
	if err == nil {
		t.Fatal("expected a non-string body to be refused")
	}
}
