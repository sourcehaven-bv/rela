package dataentry

import "testing"

// resolveLinkTarget is a closed allowlist, and that is load-bearing: its output
// is rendered into an `href` by the SPA (TKT-3CSZRG made list rows real links so
// cmd/middle-click opens a tab). An `href` is a far more dangerous sink than the
// `router.push` it replaced — a scheme-bearing value there would be an XSS.
//
// Nothing in the current code can produce one, which is exactly why this test
// exists: it fails if someone later adds a passthrough branch, rather than
// leaving the SPA's defense-in-depth check as the only thing standing between a
// config value and a `javascript:` URL.
func TestResolveLinkTarget_RejectsEverythingOutsideTheAllowlist(t *testing.T) {
	hostile := []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"  javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"//evil.example.com",
		"https://evil.example.com",
		"http://evil.example.com",
		"vbscript:msgbox(1)",
		"/entity/other/INJECTED",
		"../../etc/passwd",
		"detail/../..",
		"Detail",
		"DOCUMENT/x",
	}

	for _, link := range hostile {
		t.Run(link, func(t *testing.T) {
			if got := resolveLinkTarget(link, "ticket", "TKT-1"); got != "" {
				t.Fatalf("resolveLinkTarget(%q) = %q, want \"\" — only \"detail\" and \"document/*\" may resolve", link, got)
			}
		})
	}
}

func TestResolveLinkTarget_AllowlistedShapes(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{name: "empty", link: "", want: ""},
		{name: "detail", link: "detail", want: "/entity/ticket/TKT-1"},
		{name: "document", link: "document/spec", want: "/document/spec/TKT-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveLinkTarget(tc.link, "ticket", "TKT-1")
			if got != tc.want {
				t.Fatalf("resolveLinkTarget(%q) = %q, want %q", tc.link, got, tc.want)
			}
			// Every non-empty result must be a same-origin absolute path: one
			// leading slash, never protocol-relative.
			if got != "" && (got[0] != '/' || (len(got) > 1 && got[1] == '/')) {
				t.Fatalf("resolveLinkTarget(%q) = %q, which is not a single-slash absolute path", tc.link, got)
			}
		})
	}
}
