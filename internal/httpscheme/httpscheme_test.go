package httpscheme

import "testing"

func TestIsHTTP(t *testing.T) {
	tests := []struct {
		scheme string
		want   bool
	}{
		{"http", true},
		{"https", true},
		{"HTTP", false},
		{"ftp", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.scheme, func(t *testing.T) {
			if got := IsHTTP(tc.scheme); got != tc.want {
				t.Errorf("IsHTTP(%q) = %v, want %v", tc.scheme, got, tc.want)
			}
		})
	}
}
