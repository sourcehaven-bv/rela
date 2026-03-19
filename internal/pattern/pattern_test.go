package pattern

import (
	"errors"
	"regexp"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"empty", "", false},
		{"simple", "hello", false},
		{"single wildcard", "hello*", false},
		{"ten wildcards", "a*b*c*d*e*f*g*h*i*j*", false},
		{"eleven wildcards", "a*b*c*d*e*f*g*h*i*j*k*", true},
		{"escaped wildcards don't count", `a\*b\*c\*d\*e\*f\*g\*h\*i\*j\*k\*`, false},
		{"mixed escaped and unescaped", `a\*b*c\*d*`, false},
		{"question marks count", "a?b?c?d?e?f?g?h?i?j?k?", true},
		{"unicode", "hello*世界*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				if !errors.Is(err, ErrPatternTooComplex) {
					t.Errorf("expected ErrPatternTooComplex, got %v", err)
				}
			}
		})
	}
}

func TestGlobToAnchoredRegex(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		input   string
		matches bool
	}{
		{"exact match", "hello", "hello", true},
		{"exact no match", "hello", "world", false},
		{"trailing wildcard", "hello*", "hello world", true},
		{"trailing wildcard exact", "hello*", "hello", true},
		{"leading wildcard", "*world", "hello world", true},
		{"middle wildcard", "hello*world", "hello big world", true},
		{"single char wildcard", "te?t", "test", true},
		{"single char wildcard no match", "te?t", "teast", false},
		{"escaped asterisk", `test\*`, "test*", true},
		{"escaped asterisk no match", `test\*`, "testing", false},
		{"escaped question", `test\?`, "test?", true},
		{"escaped backslash", `test\\`, "test\\", true},
		{"escaped backslash before wildcard", `test\\*`, "test\\foo", true},
		{"unicode", "hello*世界", "hello big 世界", true},
		{"regex special chars escaped", "foo.bar", "foo.bar", true},
		{"regex special chars no match", "foo.bar", "fooXbar", false},
		{"regex brackets escaped", "[test]", "[test]", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := GlobToAnchoredRegex(tt.glob)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("failed to compile regex %q: %v", pattern, err)
			}
			if got := re.MatchString(tt.input); got != tt.matches {
				t.Errorf("GlobToAnchoredRegex(%q) pattern %q match %q = %v, want %v",
					tt.glob, pattern, tt.input, got, tt.matches)
			}
		})
	}
}

func TestGlobToSubstringRegex(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		input   string
		matches bool
	}{
		{"substring match", "hello", "say hello world", true},
		{"trailing wildcard substring", "auth*", "user authentication system", true},
		{"single char substring", "te?t", "this is a test case", true},
		{"no match", "xyz", "hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := GlobToSubstringRegex(tt.glob)
			re, err := regexp.Compile("(?i)" + pattern) // case-insensitive like search
			if err != nil {
				t.Fatalf("failed to compile regex %q: %v", pattern, err)
			}
			if got := re.MatchString(tt.input); got != tt.matches {
				t.Errorf("GlobToSubstringRegex(%q) pattern %q match %q = %v, want %v",
					tt.glob, pattern, tt.input, got, tt.matches)
			}
		})
	}
}

func TestContainsWildcard(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", false},
		{"hello*", true},
		{"hello?", true},
		{`hello\*`, false},
		{`hello\?`, false},
		{`hello\\*`, true}, // escaped backslash, then unescaped *
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ContainsWildcard(tt.input); got != tt.want {
				t.Errorf("ContainsWildcard(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStartsWithWildcard(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"*hello", true},
		{"?hello", true},
		{"hello*", false},
		{"hello", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := StartsWithWildcard(tt.input); got != tt.want {
				t.Errorf("StartsWithWildcard(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitFuzzyWildcard(t *testing.T) {
	tests := []struct {
		name         string
		pattern      string
		wantFuzzy    string
		wantWildcard string
		wantHas      bool
	}{
		{"simple trailing", "foo*", "foo", "*", true},
		{"simple question", "foo?", "foo", "?", true},
		{"no wildcard", "foobar", "foobar", "", false},
		{"wildcard with suffix", "foo*bar", "foo", "*bar", true},
		{"escaped wildcard", `foo\*bar`, "foo*bar", "", false},
		{"escaped then real", `foo\*bar*`, "foo*bar", "*", true},
		{"escaped backslash before wildcard", `foo\\*`, "foo\\", "*", true},
		{"unicode", "日本*語", "日本", "*語", true},
		{"empty", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fuzzy, wildcard, has := SplitFuzzyWildcard(tt.pattern)
			if fuzzy != tt.wantFuzzy || wildcard != tt.wantWildcard || has != tt.wantHas {
				t.Errorf("SplitFuzzyWildcard(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.pattern, fuzzy, wildcard, has, tt.wantFuzzy, tt.wantWildcard, tt.wantHas)
			}
		})
	}
}

func TestCompile(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		wantGlob bool
		wantErr  bool
	}{
		{"no wildcards", "hello", false, false},
		{"with wildcard", "hello*", true, false},
		{"too many wildcards", "a*b*c*d*e*f*g*h*i*j*k*", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, isGlob, err := Compile(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compile(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
			if isGlob != tt.wantGlob {
				t.Errorf("Compile(%q) isGlob = %v, want %v", tt.pattern, isGlob, tt.wantGlob)
			}
			if tt.wantGlob && !tt.wantErr && re == nil {
				t.Errorf("Compile(%q) returned nil regex for glob pattern", tt.pattern)
			}
		})
	}
}

func TestCompileSubstring(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr error
	}{
		{"valid pattern", "auth*", nil},
		{"leading wildcard", "*auth", ErrLeadingWildcard},
		{"too many wildcards", "a*b*c*d*e*f*g*h*i*j*k*", ErrPatternTooComplex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := CompileSubstring(tt.pattern)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("CompileSubstring(%q) expected error %v, got nil", tt.pattern, tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("CompileSubstring(%q) error = %v, want %v", tt.pattern, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("CompileSubstring(%q) unexpected error: %v", tt.pattern, err)
				}
				if re == nil {
					t.Errorf("CompileSubstring(%q) returned nil regex", tt.pattern)
				}
			}
		})
	}
}
