package metamodel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// newMetamodelWithEncryption builds a minimal Metamodel with the
// given ticket.description encrypted for `group`. Helper for tests.
func newMetamodelWithEncryption(group string) *Metamodel {
	return &Metamodel{
		Entities: map[string]EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]PropertyDef{
					"title":       {Type: "string"},
					"description": {Type: "string", Encrypted: group},
				},
			},
		},
	}
}

// newMetamodelWithEncryptedBody builds a minimal Metamodel whose
// ticket's body is encrypted for `group`.
func newMetamodelWithEncryptedBody(group string) *Metamodel {
	return &Metamodel{
		Entities: map[string]EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]PropertyDef{
					"title": {Type: "string"},
				},
				EncryptedBody: group,
			},
		},
	}
}

func TestValidateEncryption_NoEncryption_NilGroups(t *testing.T) {
	m := &Metamodel{
		Entities: map[string]EntityDef{
			"ticket": {
				Label:      "Ticket",
				Properties: map[string]PropertyDef{"title": {Type: "string"}},
			},
		},
	}
	if err := m.ValidateEncryption(nil); err != nil {
		t.Fatalf("no encryption + nil groups should be OK, got: %v", err)
	}
}

func TestValidateEncryption_Property_NilGroups(t *testing.T) {
	m := newMetamodelWithEncryption("engineering")
	err := m.ValidateEncryption(nil)
	if !errors.Is(err, ErrGroupsNotFound) {
		t.Fatalf("err = %v, want ErrGroupsNotFound", err)
	}
}

func TestValidateEncryption_Body_NilGroups(t *testing.T) {
	m := newMetamodelWithEncryptedBody("exec")
	err := m.ValidateEncryption(nil)
	if !errors.Is(err, ErrGroupsNotFound) {
		t.Fatalf("err = %v, want ErrGroupsNotFound", err)
	}
}

func TestValidateEncryption_Property_UnknownGroup(t *testing.T) {
	m := newMetamodelWithEncryption("ghost")
	g := &Groups{groups: map[string][]string{
		"engineering": {"alice"},
	}}
	err := m.ValidateEncryption(g)
	if !errors.Is(err, ErrUnknownGroup) {
		t.Fatalf("err = %v, want ErrUnknownGroup", err)
	}
	var ge *GroupError
	if !errors.As(err, &ge) {
		t.Fatal("errors.As failed")
	}
	if ge.Group != "ghost" {
		t.Errorf("Group = %q, want %q", ge.Group, "ghost")
	}
	wantPath := fmt.Sprintf("entities.%s.properties.%s", "ticket", "description")
	if ge.Path != wantPath {
		t.Errorf("Path = %q, want %q", ge.Path, wantPath)
	}
}

func TestValidateEncryption_Body_UnknownGroup(t *testing.T) {
	m := newMetamodelWithEncryptedBody("ghost")
	g := &Groups{groups: map[string][]string{
		"exec": {"bob"},
	}}
	err := m.ValidateEncryption(g)
	if !errors.Is(err, ErrUnknownGroup) {
		t.Fatalf("err = %v, want ErrUnknownGroup", err)
	}
	var ge *GroupError
	_ = errors.As(err, &ge)
	wantPath := fmt.Sprintf("entities.%s.encrypted_body", "ticket")
	if ge.Path != wantPath {
		t.Errorf("Path = %q, want %q", ge.Path, wantPath)
	}
}

func TestValidateEncryption_Property_KnownGroup(t *testing.T) {
	m := newMetamodelWithEncryption("engineering")
	g := &Groups{groups: map[string][]string{
		"engineering": {"alice", "bob"},
	}}
	if err := m.ValidateEncryption(g); err != nil {
		t.Fatalf("known group should validate, got: %v", err)
	}
}

func TestValidateEncryption_Body_KnownGroup(t *testing.T) {
	m := newMetamodelWithEncryptedBody("exec")
	g := &Groups{groups: map[string][]string{
		"exec": {"bob"},
	}}
	if err := m.ValidateEncryption(g); err != nil {
		t.Fatalf("known group should validate, got: %v", err)
	}
}

func TestValidateEncryption_MultipleEntities(t *testing.T) {
	m := &Metamodel{
		Entities: map[string]EntityDef{
			"ticket": {
				Properties: map[string]PropertyDef{
					"description": {Type: "string", Encrypted: "engineering"},
				},
			},
			"decision": {
				Properties: map[string]PropertyDef{
					"rationale": {Type: "string", Encrypted: "exec"},
				},
			},
		},
	}
	g := &Groups{groups: map[string][]string{
		"engineering": {"alice"},
		"exec":        {"bob"},
	}}
	if err := m.ValidateEncryption(g); err != nil {
		t.Fatalf("all groups valid, got: %v", err)
	}

	// Now break exec.
	g = &Groups{groups: map[string][]string{"engineering": {"alice"}}}
	err := m.ValidateEncryption(g)
	if !errors.Is(err, ErrUnknownGroup) {
		t.Fatalf("err = %v, want ErrUnknownGroup", err)
	}
}

func TestLoadWithGroups_EndToEnd_Present(t *testing.T) {
	// Minimal metamodel with an encrypted: declaration + matching
	// groups.yaml should load cleanly via LoadWithGroups.
	const mmYAML = `version: "1.0"
namespace: test
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title:
        type: string
      description:
        type: string
        encrypted: engineering
`
	const groupsYAML = `groups:
  engineering:
    - alice
    - bob
`
	fs := newMemFSWithGroups(t, groupsYAML)
	if err := fs.WriteFile("/proj/metamodel.yaml", []byte(mmYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	m, g, _, err := LoadWithGroups("/proj/metamodel.yaml", fs)
	if err != nil {
		t.Fatalf("LoadWithGroups: %v", err)
	}
	if m == nil || g == nil {
		t.Fatal("metamodel or groups nil")
	}
	ticket := m.Entities["ticket"]
	got := ticket.EncryptedProperties()
	if got["description"] != "engineering" {
		t.Errorf("EncryptedProperties[description] = %q, want engineering", got["description"])
	}
}

func TestLoadWithGroups_EndToEnd_MissingGroupsWithEncryption(t *testing.T) {
	const mmYAML = `version: "1.0"
namespace: test
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      description:
        type: string
        encrypted: engineering
`
	fs := newMemFS(t)
	if err := fs.WriteFile("/proj/metamodel.yaml", []byte(mmYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := LoadWithGroups("/proj/metamodel.yaml", fs) //nolint:dogsled // test asserts error-only contract
	if !errors.Is(err, ErrGroupsNotFound) {
		t.Fatalf("err = %v, want ErrGroupsNotFound", err)
	}
}

func TestLoadWithGroups_MetamodelMissing(t *testing.T) {
	fs := newMemFS(t)
	_, _, _, err := LoadWithGroups("/proj/metamodel.yaml", fs) //nolint:dogsled // test asserts error-only contract
	if err == nil {
		t.Fatal("expected metamodel read error")
	}
	if errors.Is(err, ErrGroupsNotFound) {
		t.Error("metamodel-read error must not be classified as ErrGroupsNotFound")
	}
}

// pathFailFS delegates to MemFS, except ReadFile of pathFail returns
// pathErr. Lets a test fail one specific path while letting others
// succeed.
type pathFailFS struct {
	storage.FS
	pathFail string
	pathErr  error
}

func (f *pathFailFS) ReadFile(p string) ([]byte, error) {
	if p == f.pathFail {
		return nil, f.pathErr
	}
	return f.FS.ReadFile(p)
}

func TestLoadWithGroups_GroupsReadError(t *testing.T) {
	// Valid metamodel, but the FS returns a non-ENOENT error on
	// groups.yaml. LoadWithGroups must surface it rather than
	// classifying as NotFound.
	const mmYAML = `version: "1.0"
namespace: test
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title:
        type: string
`
	base := newMemFS(t)
	if err := base.WriteFile("/proj/metamodel.yaml", []byte(mmYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	fs := &pathFailFS{FS: base, pathFail: "/proj/groups.yaml", pathErr: errors.New("boom")}
	_, _, _, err := LoadWithGroups("/proj/metamodel.yaml", fs) //nolint:dogsled // test asserts error-only contract
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrGroupsNotFound) {
		t.Errorf("non-ENOENT must not be classified as NotFound; got: %v", err)
	}
}

func TestLoadWithGroups_EndToEnd_MissingGroupsNoEncryption(t *testing.T) {
	// No encryption declared; missing groups.yaml must not error.
	const mmYAML = `version: "1.0"
namespace: test
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title:
        type: string
`
	fs := newMemFS(t)
	if err := fs.WriteFile("/proj/metamodel.yaml", []byte(mmYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	m, g, _, err := LoadWithGroups("/proj/metamodel.yaml", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("metamodel nil")
	}
	if g != nil {
		t.Error("groups should be nil when no groups.yaml exists")
	}
}
