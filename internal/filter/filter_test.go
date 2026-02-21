package filter

import (
	"regexp"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantProp string
		wantOp   Operator
		wantVal  string
		wantGlob bool
		wantErr  bool
	}{
		// Basic operators
		{
			name:     "equality",
			input:    "status=draft",
			wantProp: "status",
			wantOp:   OpEqual,
			wantVal:  "draft",
		},
		{
			name:     "not equal",
			input:    "status!=draft",
			wantProp: "status",
			wantOp:   OpNotEqual,
			wantVal:  "draft",
		},
		{
			name:     "less than",
			input:    "valid_until<2025-02-01",
			wantProp: "valid_until",
			wantOp:   OpLess,
			wantVal:  "2025-02-01",
		},
		{
			name:     "less than or equal",
			input:    "valid_until<=2025-02-01",
			wantProp: "valid_until",
			wantOp:   OpLessEqual,
			wantVal:  "2025-02-01",
		},
		{
			name:     "greater than",
			input:    "risk_score>5",
			wantProp: "risk_score",
			wantOp:   OpGreater,
			wantVal:  "5",
		},
		{
			name:     "greater than or equal",
			input:    "risk_score>=5",
			wantProp: "risk_score",
			wantOp:   OpGreaterEqual,
			wantVal:  "5",
		},
		{
			name:     "regex",
			input:    "title=~access.*policy",
			wantProp: "title",
			wantOp:   OpRegex,
			wantVal:  "access.*policy",
		},

		// Glob patterns
		{
			name:     "glob pattern",
			input:    "iso27001=A.9.*",
			wantProp: "iso27001",
			wantOp:   OpEqual,
			wantVal:  "A.9.*",
			wantGlob: true,
		},

		// Whitespace handling
		{
			name:     "with spaces",
			input:    "  status = draft  ",
			wantProp: "status",
			wantOp:   OpEqual,
			wantVal:  "draft",
		},

		// Edge cases
		{
			name:     "empty value",
			input:    "description=",
			wantProp: "description",
			wantOp:   OpEqual,
			wantVal:  "",
		},
		{
			name:     "value with equals",
			input:    "formula=a=b+c",
			wantProp: "formula",
			wantOp:   OpEqual,
			wantVal:  "a=b+c",
		},

		// Errors
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing operator",
			input:   "status",
			wantErr: true,
		},
		{
			name:    "missing property",
			input:   "=value",
			wantErr: true,
		},
		{
			name:    "invalid regex",
			input:   "title=~[invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
				return
			}
			if f.Property != tt.wantProp {
				t.Errorf("Parse(%q).Property = %q, want %q", tt.input, f.Property, tt.wantProp)
			}
			if f.Operator != tt.wantOp {
				t.Errorf("Parse(%q).Operator = %v, want %v", tt.input, f.Operator, tt.wantOp)
			}
			if f.Value != tt.wantVal {
				t.Errorf("Parse(%q).Value = %q, want %q", tt.input, f.Value, tt.wantVal)
			}
			if f.IsGlob != tt.wantGlob {
				t.Errorf("Parse(%q).IsGlob = %v, want %v", tt.input, f.IsGlob, tt.wantGlob)
			}
		})
	}
}

func TestParseAll(t *testing.T) {
	filters, err := ParseAll([]string{"status=draft", "priority=high"})
	if err != nil {
		t.Fatalf("ParseAll unexpected error: %v", err)
	}
	if len(filters) != 2 {
		t.Errorf("ParseAll got %d filters, want 2", len(filters))
	}

	// Test error propagation
	_, err = ParseAll([]string{"status=draft", "invalid"})
	if err == nil {
		t.Error("ParseAll expected error for invalid filter")
	}
}

func TestGlobToRegex(t *testing.T) {
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
				t.Errorf("GlobToRegex(%q).MatchString(%q) = %v, want %v (pattern: %s)", tt.glob, tt.input, got, tt.matches, pattern)
			}
		})
	}
}

func TestOperatorString(t *testing.T) {
	tests := []struct {
		op   Operator
		want string
	}{
		{OpEqual, "="},
		{OpNotEqual, "!="},
		{OpLess, "<"},
		{OpLessEqual, "<="},
		{OpGreater, ">"},
		{OpGreaterEqual, ">="},
		{OpRegex, "=~"},
	}

	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("Operator(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestMatchValue(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		op    Operator
		want  string
		match bool
	}{
		{
			name:  "string equality",
			value: "draft",
			op:    OpEqual,
			want:  "draft",
			match: true,
		},
		{
			name:  "string not equal - match",
			value: "draft",
			op:    OpNotEqual,
			want:  "published",
			match: true,
		},
		{
			name:  "string not equal - no match",
			value: "draft",
			op:    OpNotEqual,
			want:  "draft",
			match: false,
		},
		{
			name:  "integer equality",
			value: 42,
			op:    OpEqual,
			want:  "42",
			match: true,
		},
		{
			name:  "bool equality",
			value: true,
			op:    OpEqual,
			want:  "true",
			match: true,
		},
		{
			name:  "float equality",
			value: 3.14,
			op:    OpEqual,
			want:  "3.140000",
			match: true,
		},
		{
			name:  "string less than",
			value: "aaa",
			op:    OpLess,
			want:  "bbb",
			match: true,
		},
		{
			name:  "string greater than",
			value: "bbb",
			op:    OpGreater,
			want:  "aaa",
			match: true,
		},
		{
			name:  "string less than or equal",
			value: "aaa",
			op:    OpLessEqual,
			want:  "aaa",
			match: true,
		},
		{
			name:  "string greater than or equal",
			value: "bbb",
			op:    OpGreaterEqual,
			want:  "bbb",
			match: true,
		},
		// List types (multi-select)
		{
			name:  "[]string contains matching value",
			value: []string{"client", "provider"},
			op:    OpEqual,
			want:  "client",
			match: true,
		},
		{
			name:  "[]string contains second value",
			value: []string{"client", "provider"},
			op:    OpEqual,
			want:  "provider",
			match: true,
		},
		{
			name:  "[]string does not contain value",
			value: []string{"client", "provider"},
			op:    OpEqual,
			want:  "employee",
			match: false,
		},
		{
			name:  "[]string not equal - none match",
			value: []string{"client", "provider"},
			op:    OpNotEqual,
			want:  "employee",
			match: true,
		},
		{
			name:  "[]string not equal - one matches",
			value: []string{"client", "provider"},
			op:    OpNotEqual,
			want:  "client",
			match: false,
		},
		{
			name:  "[]interface{} contains matching value",
			value: []interface{}{"admin", "editor"},
			op:    OpEqual,
			want:  "editor",
			match: true,
		},
		{
			name:  "[]interface{} does not contain value",
			value: []interface{}{"admin", "editor"},
			op:    OpEqual,
			want:  "viewer",
			match: false,
		},
		{
			name:  "empty []string matches empty filter",
			value: []string{},
			op:    OpEqual,
			want:  "",
			match: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Filter{
				Property: "test",
				Operator: tt.op,
				Value:    tt.want,
			}
			got := MatchValue(tt.value, f)
			if got != tt.match {
				t.Errorf("MatchValue() = %v, want %v", got, tt.match)
			}
		})
	}
}
