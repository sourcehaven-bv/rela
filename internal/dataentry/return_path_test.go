package dataentry

import "testing"

func TestIsSafeReturnPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Happy path.
		{name: "simple path", in: "/entity/x/Y", want: "/entity/x/Y"},
		{name: "with query", in: "/list/all?status=open", want: "/list/all?status=open"},
		{name: "with fragment", in: "/doc/x#section", want: "/doc/x#section"},
		{name: "path + query + fragment", in: "/form/x?y=1#sec", want: "/form/x?y=1#sec"},

		// Open-redirect payloads — all must return "".
		{name: "protocol-relative", in: "//evil.com/pwn", want: ""},
		{name: "backslash literal", in: `/\evil.com`, want: ""},
		{name: "percent-encoded backslash", in: "/%5Cevil.com", want: ""},
		{name: "percent-encoded slash", in: "/%2Fevil.com", want: ""},
		{name: "http scheme", in: "http://evil.com", want: ""},
		{name: "https scheme", in: "https://evil.com", want: ""},
		{name: "mailto", in: "mailto:evil@evil.com", want: ""},
		{name: "javascript scheme", in: "javascript:alert(1)", want: ""},
		{name: "data scheme", in: "data:text/html,<x>", want: ""},
		{name: "no leading slash", in: "evil.com", want: ""},
		{name: "empty", in: "", want: ""},
		{name: "just slash", in: "/", want: "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isSafeReturnPath(tc.in)
			if got != tc.want {
				t.Errorf("isSafeReturnPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
