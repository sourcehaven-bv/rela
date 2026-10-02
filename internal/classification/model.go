package classification

import (
	"fmt"
	"regexp"
)

// FileName is the classification file's name, relative to the project root.
const FileName = "classification.yaml"

// Limits keep parsing and analysis bounded. Exceeding one is a lint error.
const (
	// MaxFileSize is the largest accepted classification file, in bytes.
	MaxFileSize = 1 << 20
	// MaxLabels is the largest number of labels a file may define.
	MaxLabels = 256
	// MaxRuleDepth is the deepest nesting of any_of/all_of/count in a rule.
	MaxRuleDepth = 8
	// MaxSubjectHops is the largest accepted subject_hops override.
	MaxSubjectHops = 3
	// maxCountMin bounds count.min; no realistic rule needs more.
	maxCountMin = 64
)

// Reserved field states. They are written as bare scalars instead of a label
// list, so they cannot also be label names.
const (
	StateNone        = "none"
	StateNeedsReview = "needs-review"
)

// Role says how a kind of data identifies a person.
type Role string

// The identifier roles. An empty role means the label is not about persons.
const (
	RoleNone             Role = ""
	RoleDirectIdentifier Role = "direct-identifier"
	RoleQuasiIdentifier  Role = "quasi-identifier"
	RoleAttribute        Role = "attribute"
)

func parseRole(s string) (Role, bool) {
	switch r := Role(s); r {
	case RoleDirectIdentifier, RoleQuasiIdentifier, RoleAttribute:
		return r, true
	case RoleNone:
	}
	return RoleNone, false
}

// Scope says where fields must co-occur for a combination rule to match.
type Scope string

// The evaluable scopes.
const (
	// ScopeRecord: the fields are on one entity or relation.
	ScopeRecord Scope = "record"
	// ScopeSubject: the fields are reachable from one data subject.
	ScopeSubject Scope = "subject"
)

var labelNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// reservedName reports whether name may not be used as a label: the field
// states and the role names, which appear in the same positions.
func reservedName(name string) bool {
	switch name {
	case StateNone, StateNeedsReview:
		return true
	}
	_, isRole := parseRole(name)
	return isRole
}

// Label is one entry under `labels:`.
type Label struct {
	Name        string
	Role        Role
	Description string
	Reference   string
	// Meta is free-form operator data. rela does not interpret it.
	Meta map[string]any
	// When is nil for a plain label and set for a derived label.
	When *Rule
	Line int
}

// FieldState is the review state of one field entry.
type FieldState int

// The three states a field entry can be in.
const (
	// Labeled: reviewed, carries Labels.
	Labeled FieldState = iota
	// None: reviewed, carries no label.
	None
	// NeedsReview: not reviewed yet. Lint fails on it.
	NeedsReview
)

// Assignment is the entry for one field.
type Assignment struct {
	State  FieldState
	Labels []string
	Line   int
}

// TypeAssignments maps field name to its entry, for one type.
type TypeAssignments struct {
	Fields map[string]Assignment
	Line   int
}

// Overrides correct subject inference.
type Overrides struct {
	// Subject forces an entity type to be (true) or not be (false) a subject.
	Subject map[string]bool
	// SubjectLink forces a relation type to be or not be a subject link.
	SubjectLink map[string]bool
	// SubjectHops sets the reach through a relation type (default 1).
	SubjectHops map[string]int
	// Lines records where each override key was declared, keyed
	// "<section>.<name>", for lint messages.
	Lines map[string]int
}

// File is a parsed classification file.
type File struct {
	Labels map[string]*Label
	// LabelOrder is the declaration order of Labels.
	LabelOrder      []string
	Assign          map[string]TypeAssignments
	AssignRelations map[string]TypeAssignments
	Overrides       Overrides
}

func newFile() *File {
	return &File{
		Labels:          map[string]*Label{},
		Assign:          map[string]TypeAssignments{},
		AssignRelations: map[string]TypeAssignments{},
		Overrides: Overrides{
			Subject:     map[string]bool{},
			SubjectLink: map[string]bool{},
			SubjectHops: map[string]int{},
			Lines:       map[string]int{},
		},
	}
}

// Issue codes. They are stable identifiers for tests and JSON consumers.
const (
	CodeSyntax        = "syntax"
	CodeUnknownKey    = "unknown-key"
	CodeDuplicateKey  = "duplicate-key"
	CodeYAMLFeature   = "unsupported-yaml"
	CodeInvalidValue  = "invalid-value"
	CodeInvalidName   = "invalid-name"
	CodeReservedName  = "reserved-name"
	CodeUndefined     = "undefined-label"
	CodeInvalidRule   = "invalid-rule"
	CodeLimit         = "limit-exceeded"
	CodeMissingEntry  = "missing-entry"
	CodeNeedsReview   = "needs-review"
	CodeStaleEntry    = "stale-entry"
	CodeUnknownTarget = "unknown-target"
	CodeNoEffect      = "no-effect"
)

// Issue is one problem in a classification file. Every issue is an error:
// the file is operator config, and ignoring part of it would silently drop a
// declaration.
type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	if i.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", FileName, i.Line, i.Path, i.Message)
	}
	return fmt.Sprintf("%s: %s: %s", FileName, i.Path, i.Message)
}
