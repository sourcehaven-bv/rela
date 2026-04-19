package metamodel

import "fmt"

// GroupErrorKind classifies a groups.yaml or encrypted: validation
// failure. Callers use errors.Is against the exported sentinels below;
// errors.As against *GroupError unlocks structured context (Group,
// Path, Identity).
type GroupErrorKind string

const (
	// GroupErrorNotFound indicates groups.yaml is missing and at least
	// one `encrypted:` declaration exists.
	GroupErrorNotFound GroupErrorKind = "not_found"

	// GroupErrorUnknown indicates an `encrypted:` declaration
	// references a group that is not defined in groups.yaml. Path
	// carries the YAML-path-like location, e.g.
	// "entities.ticket.properties.description".
	GroupErrorUnknown GroupErrorKind = "unknown"

	// GroupErrorDuplicate indicates the same identity appears more
	// than once inside a single group. Identity carries the offending
	// name.
	GroupErrorDuplicate GroupErrorKind = "duplicate_identity"

	// GroupErrorInvalid indicates a structural problem in groups.yaml
	// other than duplication: empty group name, empty or
	// whitespace-only identity, identity with surrounding whitespace.
	// Group + Identity carry whatever context is available.
	GroupErrorInvalid GroupErrorKind = "invalid"
)

// GroupError is the typed error returned by LoadGroups and
// ValidateEncryption.
type GroupError struct {
	Kind     GroupErrorKind
	Group    string // e.g. "engineering"; empty for NotFound
	Path     string // e.g. "entities.ticket.properties.description"; empty for file-level errors
	Identity string // e.g. "alice"; populated for Duplicate only
}

func (e *GroupError) Error() string {
	switch e.Kind {
	case GroupErrorNotFound:
		return fmt.Sprintf("metamodel: %s not found", GroupsFileName)
	case GroupErrorUnknown:
		return fmt.Sprintf("metamodel: unknown group %q at %s", e.Group, e.Path)
	case GroupErrorDuplicate:
		return fmt.Sprintf("metamodel: duplicate identity %q in group %q", e.Identity, e.Group)
	case GroupErrorInvalid:
		if e.Identity != "" {
			return fmt.Sprintf("metamodel: invalid identity %q in group %q", e.Identity, e.Group)
		}
		return fmt.Sprintf("metamodel: invalid group name %q", e.Group)
	default:
		return fmt.Sprintf("metamodel: group error (%s)", e.Kind)
	}
}

// Is supports errors.Is matching against the sentinel variables below.
// Two GroupErrors match if they share the same Kind.
func (e *GroupError) Is(target error) bool {
	t, ok := target.(*GroupError)
	if !ok {
		return false
	}
	return e.Kind == t.Kind
}

// Sentinel matchers. Callers use errors.Is(err, ErrUnknownGroup) etc.
// to branch on failure mode without inspecting the full struct.
var (
	ErrGroupsNotFound    = &GroupError{Kind: GroupErrorNotFound}
	ErrUnknownGroup      = &GroupError{Kind: GroupErrorUnknown}
	ErrDuplicateIdentity = &GroupError{Kind: GroupErrorDuplicate}
	ErrInvalidIdentity   = &GroupError{Kind: GroupErrorInvalid}
)
