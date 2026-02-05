package natsort

import (
	"testing"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		// Equal strings
		{"", "", 0},
		{"abc", "abc", 0},
		{"123", "123", 0},

		// Pure text (case-insensitive)
		{"abc", "def", -1},
		{"def", "abc", 1},
		{"abc", "ABC", 1},  // lowercase after uppercase for same letters
		{"ABC", "abc", -1}, // uppercase before lowercase

		// Pure numbers
		{"1", "2", -1},
		{"2", "1", 1},
		{"9", "10", -1},
		{"10", "9", 1},
		{"100", "99", 1},

		// Prefix with numbers — the core use case
		{"REQ-1", "REQ-2", -1},
		{"REQ-2", "REQ-10", -1},
		{"REQ-10", "REQ-2", 1},
		{"REQ-9", "REQ-10", -1},
		{"REQ-10", "REQ-11", -1},
		{"REQ-1", "REQ-1", 0},

		// Different prefixes
		{"COMP-1", "REQ-1", -1},
		{"REQ-1", "COMP-1", 1},

		// Multiple number segments
		{"a1b2", "a1b10", -1},
		{"a1b10", "a1b2", 1},
		{"v1.2.3", "v1.10.1", -1},
		{"v1.10.1", "v1.2.3", 1},
		{"v2.0.0", "v10.0.0", -1},

		// Leading zeros
		{"REQ-01", "REQ-1", 1},  // "01" has a leading zero, "1" does not
		{"REQ-1", "REQ-01", -1}, // fewer leading zeros first
		{"REQ-001", "REQ-01", 1},
		{"REQ-001", "REQ-1", 1},

		// Numerically equal with different zero padding
		{"001", "1", 1},
		{"01", "1", 1},
		{"00", "0", 1},

		// Empty vs non-empty
		{"", "a", -1},
		{"a", "", 1},
		{"", "1", -1},

		// Digits vs text at same position
		{"a1", "ab", -1}, // digit sorts before text
		{"ab", "a1", 1},

		// Length differences
		{"abc", "abcd", -1},
		{"abcd", "abc", 1},

		// Real-world IDs
		{"COMP-1", "COMP-2", -1},
		{"COMP-2", "COMP-10", -1},
		{"DEC-1", "DEC-100", -1},
		{"SOL-9", "SOL-10", -1},
		{"SOL-10", "SOL-100", -1},

		// Case insensitivity for text parts
		{"req-1", "REQ-2", -1},
		{"REQ-1", "req-2", -1},

		// All zeros
		{"0", "0", 0},
		{"00", "00", 0},
		{"0", "1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := Compare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLess(t *testing.T) {
	if !Less("REQ-2", "REQ-10") {
		t.Error("Less(REQ-2, REQ-10) should be true")
	}
	if Less("REQ-10", "REQ-2") {
		t.Error("Less(REQ-10, REQ-2) should be false")
	}
	if Less("REQ-1", "REQ-1") {
		t.Error("Less(REQ-1, REQ-1) should be false")
	}
}

func TestStrings(t *testing.T) {
	input := []string{
		"REQ-1", "REQ-10", "REQ-2", "REQ-20", "REQ-3", "REQ-11",
	}
	want := []string{
		"REQ-1", "REQ-2", "REQ-3", "REQ-10", "REQ-11", "REQ-20",
	}

	Strings(input)

	for i, got := range input {
		if got != want[i] {
			t.Errorf("Strings()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestStrings_MixedTypes(t *testing.T) {
	input := []string{
		"COMP-10", "REQ-2", "COMP-2", "REQ-1", "COMP-1", "REQ-10",
	}
	want := []string{
		"COMP-1", "COMP-2", "COMP-10", "REQ-1", "REQ-2", "REQ-10",
	}

	Strings(input)

	for i, got := range input {
		if got != want[i] {
			t.Errorf("Strings()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestStrings_Versions(t *testing.T) {
	input := []string{"v1.10.0", "v1.2.0", "v1.1.0", "v2.0.0", "v1.9.0"}
	want := []string{"v1.1.0", "v1.2.0", "v1.9.0", "v1.10.0", "v2.0.0"}

	Strings(input)

	for i, got := range input {
		if got != want[i] {
			t.Errorf("Strings()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestStrings_Empty(_ *testing.T) {
	var input []string
	Strings(input) // should not panic
}

func TestStrings_Single(t *testing.T) {
	input := []string{"one"}
	Strings(input)
	if input[0] != "one" {
		t.Errorf("single element changed: %q", input[0])
	}
}

func TestCompare_Symmetry(t *testing.T) {
	pairs := [][2]string{
		{"REQ-1", "REQ-2"},
		{"a", "b"},
		{"1", "2"},
		{"v1.2", "v1.10"},
		{"", "a"},
	}
	for _, p := range pairs {
		ab := Compare(p[0], p[1])
		ba := Compare(p[1], p[0])
		if ab != -ba {
			t.Errorf("Compare(%q,%q)=%d but Compare(%q,%q)=%d (not symmetric)",
				p[0], p[1], ab, p[1], p[0], ba)
		}
	}
}

func TestCompare_Transitivity(t *testing.T) {
	// If a < b and b < c, then a < c
	triples := [][3]string{
		{"REQ-1", "REQ-2", "REQ-10"},
		{"a", "b", "c"},
		{"COMP-1", "COMP-10", "REQ-1"},
	}
	for _, tr := range triples {
		ab := Compare(tr[0], tr[1])
		bc := Compare(tr[1], tr[2])
		ac := Compare(tr[0], tr[2])
		if ab < 0 && bc < 0 && ac >= 0 {
			t.Errorf("transitivity violated: %q < %q < %q but Compare(%q,%q)=%d",
				tr[0], tr[1], tr[2], tr[0], tr[2], ac)
		}
	}
}

func FuzzCompare(f *testing.F) {
	f.Add("REQ-1", "REQ-10")
	f.Add("", "")
	f.Add("abc", "def")
	f.Add("123", "456")
	f.Add("a1b2c3", "a1b2c4")
	f.Add("ABC", "abc")
	f.Add("00", "0")
	f.Add("a", "1")

	f.Fuzz(func(t *testing.T, a, b string) {
		ab := Compare(a, b)
		ba := Compare(b, a)

		// Antisymmetry: Compare(a,b) == -Compare(b,a)
		if ab != -ba {
			t.Errorf("antisymmetry: Compare(%q,%q)=%d, Compare(%q,%q)=%d",
				a, b, ab, b, a, ba)
		}

		// Reflexivity: Compare(a,a) == 0
		if Compare(a, a) != 0 {
			t.Errorf("reflexivity: Compare(%q,%q)=%d", a, a, Compare(a, a))
		}

		// Consistency: result is always -1, 0, or 1
		if ab < -1 || ab > 1 {
			t.Errorf("Compare(%q,%q) = %d, want -1/0/1", a, b, ab)
		}
	})
}

func FuzzTransitivity(f *testing.F) {
	f.Add("REQ-1", "REQ-2", "REQ-10")
	f.Add("a", "b", "c")
	f.Add("1", "2", "10")
	f.Add("COMP-1", "COMP-10", "REQ-1")
	f.Add("", "a", "b")
	f.Add("A", "a", "B")
	f.Add("01", "1", "2")
	f.Add("v1.2", "v1.10", "v2.0")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		ab := Compare(a, b)
		bc := Compare(b, c)
		ac := Compare(a, c)

		// Transitivity: if a <= b and b <= c, then a <= c
		if ab <= 0 && bc <= 0 && ac > 0 {
			t.Errorf("transitivity violated: Compare(%q,%q)=%d, Compare(%q,%q)=%d, Compare(%q,%q)=%d",
				a, b, ab, b, c, bc, a, c, ac)
		}

		// Reverse transitivity: if a >= b and b >= c, then a >= c
		if ab >= 0 && bc >= 0 && ac < 0 {
			t.Errorf("reverse transitivity violated: Compare(%q,%q)=%d, Compare(%q,%q)=%d, Compare(%q,%q)=%d",
				a, b, ab, b, c, bc, a, c, ac)
		}
	})
}

func FuzzSortIdempotent(f *testing.F) {
	f.Add("REQ-10,REQ-2,REQ-1")
	f.Add("a,b,c")
	f.Add("10,2,1,20,3")
	f.Add("COMP-1,REQ-2,COMP-10,REQ-1")
	f.Add(",a,")
	f.Add("ABC,abc,Abc")

	f.Fuzz(func(t *testing.T, csv string) {
		if len(csv) > 500 {
			return // bound input size
		}

		items := splitNonEmpty(csv)
		if len(items) < 2 {
			return
		}

		// Sort once
		s1 := make([]string, len(items))
		copy(s1, items)
		Strings(s1)

		// Verify sorted: every adjacent pair should satisfy Compare <= 0
		for i := 1; i < len(s1); i++ {
			if Compare(s1[i-1], s1[i]) > 0 {
				t.Errorf("not sorted at [%d]: %q > %q (input: %v)", i, s1[i-1], s1[i], items)
			}
		}

		// Sort again — should be idempotent
		s2 := make([]string, len(s1))
		copy(s2, s1)
		Strings(s2)
		for i := range s1 {
			if s1[i] != s2[i] {
				t.Errorf("not idempotent at [%d]: %q vs %q", i, s1[i], s2[i])
			}
		}
	})
}

// splitNonEmpty splits a comma-separated string, keeping empty elements.
func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	return result
}
