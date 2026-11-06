package script

import (
	"strings"
	"testing"
)

// TestParseActionResponse_AssertedRoles covers the `asserted_roles` key an IdP
// webhook action returns (TKT-IZL6NX8): absent, empty and a list stay distinct.
func TestParseActionResponse_AssertedRoles(t *testing.T) {
	tests := []struct {
		name    string
		in      map[string]any
		want    []string
		wantNil bool
		wantErr string
	}{
		{name: "absent", in: map[string]any{"message": "ok"}, wantNil: true},
		{name: "empty table clears", in: map[string]any{"asserted_roles": map[string]any{}}, want: []string{}},
		{name: "list", in: map[string]any{"asserted_roles": []any{"cpo", "fg"}}, want: []string{"cpo", "fg"}},
		{name: "with a message", in: map[string]any{"message": "ok", "asserted_roles": []any{"cpo"}},
			want: []string{"cpo"}},
		{name: "non-string item", in: map[string]any{"asserted_roles": []any{"cpo", int64(1)}},
			wantErr: "list of strings"},
		{name: "not a list", in: map[string]any{"asserted_roles": "cpo"}, wantErr: "list of strings"},
		{name: "keyed table", in: map[string]any{"asserted_roles": map[string]any{"a": "cpo"}},
			wantErr: "list of strings"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := parseActionResponse(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if resp.AssertedRoles != nil {
					t.Fatalf("AssertedRoles = %#v, want nil", resp.AssertedRoles)
				}
				return
			}
			if resp.AssertedRoles == nil || strings.Join(resp.AssertedRoles, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("AssertedRoles = %#v, want %#v", resp.AssertedRoles, tc.want)
			}
		})
	}
}
