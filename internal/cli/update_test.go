package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager/entitymanagertest"
)

// update_test.go covers only CLI-level concerns. The property flag
// parsing itself is verified by TestParsePropertyFlag in create_test.go
// (shared helper), and entity property mutation is covered by the
// entity package tests — no need to duplicate either here.

func TestUpdateCmd_PropertyFlagExists(t *testing.T) {
	rt := reflect.TypeFor[UpdateCmd]()
	f, ok := rt.FieldByName("Property")
	if !ok {
		t.Fatal("update command struct should have a Property field")
	}
	if got := f.Tag.Get("short"); got != "P" {
		t.Errorf("Property field short tag = %q, want %q", got, "P")
	}
}

// capturingPatcher records the patch the command built, so these tests
// assert on the WIRE SHAPE the CLI produces rather than on storage.
//
// PanicOnUse supplies the rest of the interface: any write path other
// than PatchEntity is a test failure, loudly.
type capturingPatcher struct {
	entitymanagertest.PanicOnUse
	gotID    string
	gotPatch entity.Patch
	called   int
}

func (c *capturingPatcher) PatchEntity(
	_ context.Context, id string, p entity.Patch,
) (*entity.UpdateResult, error) {
	c.called++
	c.gotID, c.gotPatch = id, p
	return &entity.UpdateResult{Entity: entity.New(id, "task")}, nil
}

// TestUpdateCmd_BuildsTargetedPatch pins the flag → patch translation,
// including the two capabilities `rela update` gained in TKT-80EWGM
// (remove a property, clear the body) and the one it deliberately did
// NOT change (-P key= still sets the empty string).
func TestUpdateCmd_BuildsTargetedPatch(t *testing.T) {
	tests := []struct {
		name       string
		cmd        UpdateCmd
		wantProps  map[string]any
		wantUnset  []string
		wantBody   *string
		wantNoBody bool
	}{
		{
			name:       "named property only — nothing else is touched",
			cmd:        UpdateCmd{ID: "TASK-1", Title: "Renamed"},
			wantProps:  map[string]any{"title": "Renamed"},
			wantNoBody: true,
		},
		{
			name:       "unset removes a property",
			cmd:        UpdateCmd{ID: "TASK-1", Unset: []string{"due"}},
			wantProps:  map[string]any{},
			wantUnset:  []string{"due"},
			wantNoBody: true,
		},
		{
			name:       "-P key= sets an EMPTY STRING, it does not remove",
			cmd:        UpdateCmd{ID: "TASK-1", Property: []string{"notes="}},
			wantProps:  map[string]any{"notes": ""},
			wantNoBody: true,
		},
		{
			name:      "clear-body sends a pointer to empty",
			cmd:       UpdateCmd{ID: "TASK-1", ClearBody: true},
			wantProps: map[string]any{},
			wantBody:  new(""),
		},
		{
			name:      "body sets a pointer to the content",
			cmd:       UpdateCmd{ID: "TASK-1", Body: "hello"},
			wantProps: map[string]any{},
			wantBody:  new("hello"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildPatch(t, tc.cmd)

			if len(got.Properties) != len(tc.wantProps) {
				t.Errorf("Properties = %v, want %v", got.Properties, tc.wantProps)
			}
			for k, want := range tc.wantProps {
				if got.Properties[k] != want {
					t.Errorf("Properties[%q] = %v, want %v", k, got.Properties[k], want)
				}
			}
			if len(got.MetaUnset) != len(tc.wantUnset) {
				t.Errorf("MetaUnset = %v, want %v", got.MetaUnset, tc.wantUnset)
			}
			switch {
			case tc.wantNoBody && got.Content != nil:
				t.Errorf("Content = %q, want nil (body untouched)", *got.Content)
			case tc.wantBody != nil && got.Content == nil:
				t.Errorf("Content = nil, want %q", *tc.wantBody)
			case tc.wantBody != nil && *got.Content != *tc.wantBody:
				t.Errorf("Content = %q, want %q", *got.Content, *tc.wantBody)
			}
		})
	}
}

// TestUpdateCmd_ClearBodyConflictsWithBody pins the guard: the two ways
// of specifying a body are mutually exclusive, and the conflict is a
// loud error rather than a silent precedence rule.
//
// The check is on the FLAGS, not the resolved content — otherwise
// `--clear-body -B empty.md` would slip through, since an empty file
// resolves to "" and would look like "no body was supplied".
func TestUpdateCmd_ClearBodyConflictsWithBody(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  UpdateCmd
	}{
		{"with --body", UpdateCmd{ID: "TASK-1", ClearBody: true, Body: "text"}},
		{"with --body-file", UpdateCmd{ID: "TASK-1", ClearBody: true, BodyFile: "notes.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &capturingPatcher{}
			err := tc.cmd.Run(context.Background(), &writeServices{
				readServices:  readServices{},
				EntityManager: p,
			})
			if err == nil {
				t.Fatal("expected an error when --clear-body is combined with a body source")
			}
			if p.called != 0 {
				t.Error("a conflicting invocation must not reach the write path")
			}
		})
	}
}

// TestUpdateCmd_EmptyBodyFileIsHonored pins that naming an empty file is
// treated as an explicit instruction, not as "no updates specified". The
// operator pointed at a source; degrading that to an error about supplying
// nothing would be baffling.
func TestUpdateCmd_EmptyBodyFileIsHonored(t *testing.T) {
	captureOut(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	p := &capturingPatcher{}
	err := (&UpdateCmd{ID: "TASK-1", BodyFile: path}).Run(
		context.Background(), &writeServices{
			readServices:  readServices{},
			EntityManager: p,
		})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if p.gotPatch.Content == nil {
		t.Fatal("Content = nil; an explicitly named body file must be honored")
	}
	if *p.gotPatch.Content != "" {
		t.Errorf("Content = %q, want empty", *p.gotPatch.Content)
	}
}

// TestUpdateCmd_NoUpdatesSpecified pins that an empty patch is refused
// rather than dispatched as a no-op write.
func TestUpdateCmd_NoUpdatesSpecified(t *testing.T) {
	p := &capturingPatcher{}

	err := (&UpdateCmd{ID: "TASK-1"}).Run(context.Background(), &writeServices{
		readServices:  readServices{},
		EntityManager: p,
	})
	if err == nil {
		t.Fatal("expected an error when no update flags were supplied")
	}
	if p.called != 0 {
		t.Error("an empty patch must not reach the write path")
	}
}

// buildPatch runs the command against a capturing fake and returns the
// patch it produced. captureOut is required because Run writes progress
// through the package-level output writer, which is nil by default in
// tests (and is a mutable global, so these tests are not parallel).
func buildPatch(t *testing.T, cmd UpdateCmd) entity.Patch {
	t.Helper()
	captureOut(t)
	p := &capturingPatcher{}
	if err := cmd.Run(context.Background(), &writeServices{
		readServices:  readServices{},
		EntityManager: p,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if p.called != 1 {
		t.Fatalf("PatchEntity called %d times, want 1", p.called)
	}
	if p.gotID != cmd.ID {
		t.Errorf("patched id = %q, want %q", p.gotID, cmd.ID)
	}
	return p.gotPatch
}
