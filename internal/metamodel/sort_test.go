package metamodel

import "testing"

func TestSortSpecIsDescending(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		want      bool
	}{
		{"empty direction", "", false},
		{"asc direction", "asc", false},
		{"desc direction", "desc", true},
		{"other value", "ascending", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := SortSpec{Property: "title", Direction: tt.direction}
			if got := s.IsDescending(); got != tt.want {
				t.Errorf("SortSpec.IsDescending() = %v, want %v", got, tt.want)
			}
		})
	}
}
