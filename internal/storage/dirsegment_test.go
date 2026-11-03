package storage

import "testing"

func TestValidateDirSegment(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"requirements", true},
		{"Items", true},
		{"salary_reviews", true},
		{"to-do lists", true},
		{"categorieën", true},
		{"v1.2", true},
		{"-dash", true},
		{"_private", true},
		{"", false},
		{".", false},
		{"..", false},
		{"../relations", false},
		{"a/b", false},
		{`a\b`, false},
		{".hidden", false},
		{"a\x00b", false},
		{"a\nb", false},
		{"a\x7fb", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDirSegment(tc.name)
			if (err == nil) != tc.ok {
				t.Fatalf("ValidateDirSegment(%q) = %v, want ok=%v", tc.name, err, tc.ok)
			}
		})
	}
}
