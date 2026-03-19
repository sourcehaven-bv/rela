package filter

import (
	"errors"
	"regexp"
	"testing"
)

func TestGlobToRegex_Unicode(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		input   string
		matches bool
	}{
		// CJK character (multi-byte UTF-8)
		{
			name:    "question mark matches CJK codepoint",
			glob:    "test?",
			input:   "test\u4e2d", // "test中"
			matches: true,
		},
		{
			name:    "question mark does not match two CJK codepoints",
			glob:    "test?",
			input:   "test\u4e2d\u6587", // "test中文"
			matches: false,
		},
		// Emoji (4-byte UTF-8)
		{
			name:    "asterisk matches emoji and more",
			glob:    "emoji*",
			input:   "emoji\U0001F600test", // "emoji😀test"
			matches: true,
		},
		{
			name:    "question mark matches single emoji",
			glob:    "emoji?test",
			input:   "emoji\U0001F600test",
			matches: true,
		},
		{
			name:    "question mark does not match two emojis",
			glob:    "emoji?test",
			input:   "emoji\U0001F600\U0001F601test",
			matches: false,
		},
		// Combining accent (multi-codepoint grapheme - treated as one ? per codepoint)
		{
			name:    "question mark matches base char with combining accent needs two",
			glob:    "a??b",
			input:   "a\u0065\u0301b", // "aéb" as e + combining acute
			matches: true,
		},
		// Mixed Unicode
		{
			name:    "asterisk with CJK prefix",
			glob:    "\u4e2d*",
			input:   "\u4e2d\u6587\u6d4b\u8bd5",
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := GlobToRegex(tt.glob)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("GlobToRegex(%q) produced invalid regex %q: %v", tt.glob, pattern, err)
			}
			got := re.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("GlobToRegex(%q).MatchString(%q) = %v, want %v (pattern: %s)",
					tt.glob, tt.input, got, tt.matches, pattern)
			}
		})
	}
}

func TestGlobToRegex_Escapes(t *testing.T) {
	tests := []struct {
		name    string
		glob    string
		input   string
		matches bool
	}{
		// Escaped asterisk
		{
			name:    "escaped asterisk matches literal asterisk",
			glob:    `file\*`,
			input:   "file*",
			matches: true,
		},
		{
			name:    "escaped asterisk does not match other text",
			glob:    `file\*`,
			input:   "filetest",
			matches: false,
		},
		// Escaped question mark
		{
			name:    "escaped question mark matches literal question",
			glob:    `what\?`,
			input:   "what?",
			matches: true,
		},
		{
			name:    "escaped question mark does not match other char",
			glob:    `what\?`,
			input:   "whata",
			matches: false,
		},
		// Escaped tilde (for future fuzzy operator)
		{
			name:    "escaped tilde matches literal tilde",
			glob:    `term\~`,
			input:   "term~",
			matches: true,
		},
		// Escaped backslash
		{
			name:    "escaped backslash matches literal backslash",
			glob:    `path\\name`,
			input:   `path\name`,
			matches: true,
		},
		// Mixed escapes and wildcards
		{
			name:    "escaped asterisk with real asterisk",
			glob:    `\*wildcard*`,
			input:   "*wildcard_test",
			matches: true,
		},
		{
			name:    "multiple escapes",
			glob:    `a\*b\?c`,
			input:   "a*b?c",
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := GlobToRegex(tt.glob)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("GlobToRegex(%q) produced invalid regex %q: %v", tt.glob, pattern, err)
			}
			got := re.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("GlobToRegex(%q).MatchString(%q) = %v, want %v (pattern: %s)",
					tt.glob, tt.input, got, tt.matches, pattern)
			}
		})
	}
}

func TestValidatePattern_ReDoS(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "10 asterisks is valid",
			pattern: "a*b*c*d*e*f*g*h*i*j",
			wantErr: false,
		},
		{
			name:    "11 asterisks is rejected",
			pattern: "a*b*c*d*e*f*g*h*i*j*k*l",
			wantErr: true,
		},
		{
			name:    "10 question marks is valid",
			pattern: "??????????",
			wantErr: false,
		},
		{
			name:    "11 question marks is rejected",
			pattern: "???????????",
			wantErr: true,
		},
		{
			name:    "mixed wildcards at limit",
			pattern: "*?*?*?*?*?", // 5 asterisks + 5 question marks = 10
			wantErr: false,
		},
		{
			name:    "mixed wildcards over limit",
			pattern: "*?*?*?*?*?*", // 6 asterisks + 5 question marks = 11
			wantErr: true,
		},
		{
			name:    "escaped wildcards do not count",
			pattern: `\*\*\*\*\*\*\*\*\*\*\*`, // 11 escaped asterisks
			wantErr: false,
		},
		{
			name:    "no wildcards is valid",
			pattern: "simple-pattern",
			wantErr: false,
		},
		{
			name:    "empty pattern is valid",
			pattern: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePattern(tt.pattern)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePattern(%q) expected error, got nil", tt.pattern)
				} else if !errors.Is(err, ErrPatternTooComplex) {
					t.Errorf("ValidatePattern(%q) error should wrap ErrPatternTooComplex, got %v", tt.pattern, err)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePattern(%q) unexpected error: %v", tt.pattern, err)
				}
			}
		})
	}
}

func TestGlobToRegex_BackwardCompat(t *testing.T) {
	// These are the exact test cases from the original TestGlobToRegex in filter_test.go
	tests := []struct {
		glob    string
		input   string
		matches bool
	}{
		{"A.9.*", "A.9.1", true},
		{"A.9.*", "A.9.1.1", true},
		{"A.9.*", "A.10.1", false},
		{"*.txt", "file.txt", true},
		{"*.txt", "file.csv", false},
		{"test?", "test1", true},
		{"test?", "test12", false},
		{"[test]", "[test]", true}, // Escapes special chars
	}

	for _, tt := range tests {
		t.Run(tt.glob+"_"+tt.input, func(t *testing.T) {
			pattern := GlobToRegex(tt.glob)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("GlobToRegex(%q) produced invalid regex %q: %v", tt.glob, pattern, err)
			}
			got := re.MatchString(tt.input)
			if got != tt.matches {
				t.Errorf("GlobToRegex(%q).MatchString(%q) = %v, want %v (pattern: %s)",
					tt.glob, tt.input, got, tt.matches, pattern)
			}
		})
	}
}

func TestParsePattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		wantGlob bool
		wantErr  bool
	}{
		{
			name:     "simple glob with asterisk",
			pattern:  "test*",
			wantGlob: true,
			wantErr:  false,
		},
		{
			name:     "simple glob with question mark",
			pattern:  "test?",
			wantGlob: true,
			wantErr:  false,
		},
		{
			name:     "no wildcards - not a glob",
			pattern:  "simple",
			wantGlob: false,
			wantErr:  false,
		},
		{
			name:     "escaped wildcards - not a glob",
			pattern:  `test\*value`,
			wantGlob: false,
			wantErr:  false,
		},
		{
			name:     "too many wildcards",
			pattern:  "a*b*c*d*e*f*g*h*i*j*k*l",
			wantGlob: true,
			wantErr:  true,
		},
		{
			name:     "mixed escaped and unescaped",
			pattern:  `\*real*`,
			wantGlob: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, isGlob, err := ParsePattern(tt.pattern)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePattern(%q) expected error, got nil", tt.pattern)
				}
				return
			}

			if err != nil {
				t.Errorf("ParsePattern(%q) unexpected error: %v", tt.pattern, err)
				return
			}

			if isGlob != tt.wantGlob {
				t.Errorf("ParsePattern(%q) isGlob = %v, want %v", tt.pattern, isGlob, tt.wantGlob)
			}

			if isGlob && re == nil {
				t.Errorf("ParsePattern(%q) returned nil regex for glob pattern", tt.pattern)
			}

			if !isGlob && re != nil {
				t.Errorf("ParsePattern(%q) returned non-nil regex for non-glob pattern", tt.pattern)
			}
		})
	}
}

func TestPatternError(t *testing.T) {
	err := &PatternError{Pattern: "test*****", Reason: "too many wildcards", Err: ErrPatternTooComplex}

	// Test Error() method
	errStr := err.Error()
	if errStr == "" {
		t.Error("PatternError.Error() returned empty string")
	}

	// Test Unwrap() method
	if !errors.Is(err, ErrPatternTooComplex) {
		t.Error("PatternError should unwrap to ErrPatternTooComplex")
	}
}

func TestSplitFuzzyWildcard(t *testing.T) {
	tests := []struct {
		name            string
		pattern         string
		wantFuzzy       string
		wantWildcard    string
		wantHasWildcard bool
	}{
		{"simple suffix wildcard", "foo*", "foo", "*", true},
		{"question mark wildcard", "auth?ize", "auth", "?ize", true},
		{"no wildcard", "foo", "foo", "", false},
		{"escaped asterisk", "test\\*val*", "test*val", "*", true},
		{"escaped question", "test\\?val?", "test?val", "?", true},
		{"multiple wildcards", "auth*ize*", "auth", "*ize*", true},
		{"wildcard at start", "*suffix", "", "*suffix", true},
		{"empty pattern", "", "", "", false},
		{"escaped backslash before wildcard", "test\\\\*", "test\\", "*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fuzzy, wildcard, hasWildcard := SplitFuzzyWildcard(tt.pattern)
			if fuzzy != tt.wantFuzzy {
				t.Errorf("fuzzy = %q, want %q", fuzzy, tt.wantFuzzy)
			}
			if wildcard != tt.wantWildcard {
				t.Errorf("wildcard = %q, want %q", wildcard, tt.wantWildcard)
			}
			if hasWildcard != tt.wantHasWildcard {
				t.Errorf("hasWildcard = %v, want %v", hasWildcard, tt.wantHasWildcard)
			}
		})
	}
}
