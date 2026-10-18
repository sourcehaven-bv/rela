package acl

import "testing"

const convertCeilingPolicy = `
roles:
  converter:
    read: ["*"]
    convert: [note]
assignments:
  converter: converter
client_baselines:
  no-task-create:
    applies_to: [no-task-create]
    deny_create: [task]
  no-note-delete:
    applies_to: [no-note-delete]
    deny_delete: [note]
  no-note-update:
    applies_to: [no-note-update]
    deny_update: [note]
  task-only:
    applies_to: [task-only]
    create: [task]
`

// TestConvert_CeilingBoundsDestinationAndSource pins that a client ceiling
// cannot be escaped through `convert:`. A conversion creates the destination
// type and ends the source type, so the ceiling's create axis on the
// destination and its update and delete axes on the source all bound it.
func TestConvert_CeilingBoundsDestinationAndSource(t *testing.T) {
	t.Parallel()
	d := mustDeclarative(t, convertCeilingPolicy)
	for _, tc := range []struct {
		name          string
		principalType string
		subject       EntitySubject
		want          bool
	}{
		{"no ceiling", "user", NewConversionSubject("note", "NOTE-1", "task"), true},
		{"no ceiling, no destination", "user", NewFamilySubject("note", "NOTE-1"), true},
		{"create withheld on destination", "no-task-create", NewConversionSubject("note", "NOTE-1", "task"), false},
		{"create permitted on other destination", "no-task-create", NewConversionSubject("note", "NOTE-1", "memo"), true},
		{"delete withheld on source", "no-note-delete", NewConversionSubject("note", "NOTE-1", "task"), false},
		{"update withheld on source", "no-note-update", NewConversionSubject("note", "NOTE-1", "task"), false},
		{"create allowlist names destination", "task-only", NewConversionSubject("note", "NOTE-1", "task"), true},
		{"create allowlist omits destination", "task-only", NewConversionSubject("note", "NOTE-1", "memo"), false},
		{"ceiling, no destination", "task-only", NewFamilySubject("note", "NOTE-1"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := OpenRequest(t.Context(), d, verifiedClient("converter", tc.principalType))
			if err != nil {
				t.Fatal(err)
			}
			got := r.AuthorizeWrite(t.Context(), WriteRequest{Op: OpConvert, Subject: tc.subject})
			if got.Allow != tc.want {
				t.Errorf("allow = %v, want %v (%s)", got.Allow, tc.want, got.Reason)
			}
		})
	}
}
