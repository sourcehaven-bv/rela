package metamodel

import (
	"errors"
	"strings"
	"testing"
)

func TestGroupError_Error_NotFound(t *testing.T) {
	e := &GroupError{Kind: GroupErrorNotFound}
	if !strings.Contains(e.Error(), GroupsFileName) {
		t.Errorf("NotFound Error() = %q, want to contain %q", e.Error(), GroupsFileName)
	}
}

func TestGroupError_Error_Unknown(t *testing.T) {
	e := &GroupError{Kind: GroupErrorUnknown, Group: "ghost", Path: "entities.x.properties.y"}
	got := e.Error()
	if !strings.Contains(got, "ghost") || !strings.Contains(got, "entities.x.properties.y") {
		t.Errorf("Unknown Error() = %q, want group + path", got)
	}
}

func TestGroupError_Error_Duplicate(t *testing.T) {
	e := &GroupError{Kind: GroupErrorDuplicate, Group: "eng", Identity: "alice"}
	got := e.Error()
	if !strings.Contains(got, "alice") || !strings.Contains(got, "eng") {
		t.Errorf("Duplicate Error() = %q, want identity + group", got)
	}
}

func TestGroupError_Error_InvalidIdentity(t *testing.T) {
	e := &GroupError{Kind: GroupErrorInvalid, Group: "eng", Identity: " alice"}
	got := e.Error()
	if !strings.Contains(got, " alice") || !strings.Contains(got, "eng") {
		t.Errorf("Invalid Error() = %q, want identity + group", got)
	}
}

func TestGroupError_Error_InvalidGroupName(t *testing.T) {
	// Empty identity → message reports the group name only (for
	// "invalid group name").
	e := &GroupError{Kind: GroupErrorInvalid, Group: ""}
	got := e.Error()
	if !strings.Contains(got, "invalid group name") {
		t.Errorf("InvalidGroupName Error() = %q, want group-name phrasing", got)
	}
}

func TestGroupError_Error_UnknownKind(t *testing.T) {
	// Defense against future unknown kind — doesn't panic, returns
	// something.
	e := &GroupError{Kind: "weird"}
	if e.Error() == "" {
		t.Error("Error() with unknown kind returned empty string")
	}
}

func TestGroupError_Is_WrongType(t *testing.T) {
	e := &GroupError{Kind: GroupErrorNotFound}
	other := errors.New("unrelated error")
	if errors.Is(e, other) {
		t.Error("GroupError should not match an unrelated error type")
	}
}

func TestGroupError_Is_DifferentKind(t *testing.T) {
	e := &GroupError{Kind: GroupErrorNotFound}
	other := &GroupError{Kind: GroupErrorUnknown}
	if errors.Is(e, other) {
		t.Error("NotFound should not match Unknown")
	}
}

func TestIsGroupsNotFound_WrongType(t *testing.T) {
	if IsGroupsNotFound(errors.New("not a group error")) {
		t.Error("IsGroupsNotFound should be false for non-GroupError")
	}
}

func TestIsGroupsNotFound_Nil(t *testing.T) {
	if IsGroupsNotFound(nil) {
		t.Error("IsGroupsNotFound(nil) should be false")
	}
}
