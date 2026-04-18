package testutil

import (
	"strings"
	"testing"
)

func TestRandomString(t *testing.T) {
	s := RandomString()
	if s == "" {
		t.Error("RandomString returned empty string")
	}
	if !strings.HasPrefix(s, "word-") {
		t.Errorf("RandomString = %q, want prefix 'word-'", s)
	}
}

func TestRandomID(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{"with prefix", "TKT", "TKT-"},
		{"empty prefix", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := RandomID(tt.prefix)
			if id == "" {
				t.Error("RandomID returned empty string")
			}
			if tt.want != "" && !strings.HasPrefix(id, tt.want) {
				t.Errorf("RandomID(%q) = %q, want prefix %q", tt.prefix, id, tt.want)
			}
		})
	}
}

func TestRandomInt(t *testing.T) {
	tests := []struct {
		name     string
		min, max int
	}{
		{"normal range", 1, 10},
		{"same value", 5, 5},
		{"swapped", 10, 1}, // should swap and work
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 100 {
				v := RandomInt(tt.min, tt.max)
				minVal, maxVal := tt.min, tt.max
				if minVal > maxVal {
					minVal, maxVal = maxVal, minVal
				}
				if v < minVal || v > maxVal {
					t.Errorf("RandomInt(%d, %d) = %d, out of range [%d, %d]",
						tt.min, tt.max, v, minVal, maxVal)
				}
			}
		})
	}
}

func TestRandomBool(t *testing.T) {
	// Run enough times to get both values
	gotTrue, gotFalse := false, false
	for range 100 {
		if RandomBool() {
			gotTrue = true
		} else {
			gotFalse = true
		}
		if gotTrue && gotFalse {
			break
		}
	}
	if !gotTrue || !gotFalse {
		t.Error("RandomBool did not return both true and false in 100 iterations")
	}
}

func TestRandomDate(t *testing.T) {
	date := RandomDate()
	if date == "" {
		t.Error("RandomDate returned empty string")
	}
	// Check format: YYYY-MM-DD
	if len(date) != 10 {
		t.Errorf("RandomDate = %q, want length 10 (YYYY-MM-DD)", date)
	}
	if date[4] != '-' || date[7] != '-' {
		t.Errorf("RandomDate = %q, want format YYYY-MM-DD", date)
	}
}

func TestRandomEnumValue(t *testing.T) {
	values := []string{"a", "b", "c"}

	// Run enough times to hit each value
	seen := make(map[string]bool)
	for range 100 {
		v := RandomEnumValue(values)
		seen[v] = true
		// Verify value is in list
		found := false
		for _, valid := range values {
			if v == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RandomEnumValue returned %q, not in %v", v, values)
		}
	}
}

func TestRandomEnumValue_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("RandomEnumValue(empty) did not panic")
		}
	}()
	RandomEnumValue([]string{})
}
