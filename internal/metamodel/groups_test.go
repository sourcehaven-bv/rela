package metamodel

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// testProjRoot is the conventional in-memory project root used by
// metamodel tests. Kept as a constant so fixtures read uniformly.
const testProjRoot = "/proj"

// newMemFS returns a MemFS with testProjRoot pre-created. Shared
// across tests in this package.
func newMemFS(t *testing.T) storage.FS {
	t.Helper()
	fs := storage.NewMemFS()
	if err := fs.MkdirAll(testProjRoot, 0o755); err != nil {
		t.Fatalf("setup MkdirAll: %v", err)
	}
	return fs
}

// newMemFSWithGroups writes groups.yaml at <testProjRoot>/groups.yaml
// in an in-memory FS with the given content.
func newMemFSWithGroups(t *testing.T, content string) storage.FS {
	t.Helper()
	fs := newMemFS(t)
	path := testProjRoot + "/" + GroupsFileName
	if err := fs.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
	return fs
}

func TestLoadGroups_Basic(t *testing.T) {
	const yaml = `groups:
  engineering:
    - charlie
    - alice
    - bob
  exec:
    - bob
    - dan
`
	fs := newMemFSWithGroups(t, yaml)
	g, err := LoadGroups("/proj", fs)
	if err != nil {
		t.Fatalf("LoadGroups: %v", err)
	}

	// Contains
	if !g.Contains("engineering") {
		t.Error("Contains(engineering) = false, want true")
	}
	if !g.Contains("exec") {
		t.Error("Contains(exec) = false, want true")
	}
	if g.Contains("ghost") {
		t.Error("Contains(ghost) = true, want false")
	}

	// Recipients preserves declaration order
	got, ok := g.Recipients("engineering")
	if !ok {
		t.Fatal("Recipients(engineering): not found")
	}
	want := []string{"charlie", "alice", "bob"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recipients(engineering) = %v, want %v (order preserved)", got, want)
	}

	if _, ok := g.Recipients("ghost"); ok {
		t.Error("Recipients(ghost) reported ok; want false")
	}
}

func TestLoadGroups_Missing(t *testing.T) {
	fs := storage.NewMemFS()
	_, err := LoadGroups("/proj", fs)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, ErrGroupsNotFound) {
		t.Fatalf("err = %v, want ErrGroupsNotFound", err)
	}
	var ge *GroupError
	if !errors.As(err, &ge) {
		t.Fatal("errors.As(*GroupError) failed")
	}
	if ge.Kind != GroupErrorNotFound {
		t.Errorf("Kind = %q, want %q", ge.Kind, GroupErrorNotFound)
	}
}

func TestLoadGroups_DuplicateIdentity(t *testing.T) {
	const yaml = `groups:
  engineering:
    - alice
    - bob
    - alice
`
	fs := newMemFSWithGroups(t, yaml)
	_, err := LoadGroups("/proj", fs)
	if !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("err = %v, want ErrDuplicateIdentity", err)
	}
	var ge *GroupError
	if !errors.As(err, &ge) {
		t.Fatal("errors.As failed")
	}
	if ge.Group != "engineering" {
		t.Errorf("Group = %q, want %q", ge.Group, "engineering")
	}
	if ge.Identity != "alice" {
		t.Errorf("Identity = %q, want %q", ge.Identity, "alice")
	}
}

func TestLoadGroups_StrictUnknownField(t *testing.T) {
	const yaml = `groups:
  engineering:
    - alice
grupes:  # typo — strict mode must reject
  exec:
    - bob
`
	fs := newMemFSWithGroups(t, yaml)
	_, err := LoadGroups("/proj", fs)
	if err == nil {
		t.Fatal("expected strict-mode rejection of unknown field")
	}
}

func TestLoadGroups_MalformedYAML(t *testing.T) {
	const bad = "groups: [this is not a map"
	fs := newMemFSWithGroups(t, bad)
	_, err := LoadGroups("/proj", fs)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoadGroups_EmptyFile(t *testing.T) {
	fs := newMemFSWithGroups(t, "")
	g, err := LoadGroups("/proj", fs)
	if err != nil {
		t.Fatalf("empty file should load cleanly, got: %v", err)
	}
	if g == nil {
		t.Fatal("g is nil")
	}
	if g.Contains("anything") {
		t.Error("empty groups.yaml must contain no groups")
	}
}

func TestLoadGroups_EmptyGroup(t *testing.T) {
	// Empty group is legal (wiring layer surfaces it as an empty
	// recipient list when trying to wrap).
	const yaml = `groups:
  engineering: []
`
	fs := newMemFSWithGroups(t, yaml)
	g, err := LoadGroups("/proj", fs)
	if err != nil {
		t.Fatalf("empty group should load: %v", err)
	}
	if !g.Contains("engineering") {
		t.Error("Contains(engineering) = false, want true")
	}
	got, ok := g.Recipients("engineering")
	if !ok {
		t.Fatal("Recipients(engineering): not found")
	}
	if len(got) != 0 {
		t.Errorf("Recipients = %v, want empty", got)
	}
}

// failingFS is a storage.FS that returns err on ReadFile of any
// path. Other methods delegate to an embedded MemFS.
type failingFS struct {
	storage.FS
	err error
}

func (f *failingFS) ReadFile(_ string) ([]byte, error) {
	return nil, f.err
}

func TestLoadGroups_ReadFileOtherError(t *testing.T) {
	// Non-ENOENT I/O error on read must surface as a wrapped error,
	// not as ErrGroupsNotFound.
	fs := &failingFS{FS: storage.NewMemFS(), err: errors.New("permission denied")}
	_, err := LoadGroups("/proj", fs)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrGroupsNotFound) {
		t.Errorf("permission error must not be classified as NotFound; got: %v", err)
	}
}

func TestLoadGroups_InvalidIdentities(t *testing.T) {
	cases := []struct {
		name, content string
	}{
		{"empty string", `groups:
  engineering:
    - alice
    - ""
`},
		{"whitespace only", `groups:
  engineering:
    - "   "
`},
		{"leading whitespace", `groups:
  engineering:
    - " alice"
`},
		{"trailing whitespace", `groups:
  engineering:
    - "alice "
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newMemFSWithGroups(t, tc.content)
			_, err := LoadGroups("/proj", fs)
			if !errors.Is(err, ErrInvalidIdentity) {
				t.Fatalf("err = %v, want ErrInvalidIdentity", err)
			}
		})
	}
}

func TestLoadGroups_EmptyGroupName(t *testing.T) {
	const yaml = `groups:
  "":
    - alice
`
	fs := newMemFSWithGroups(t, yaml)
	_, err := LoadGroups("/proj", fs)
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("err = %v, want ErrInvalidIdentity", err)
	}
}

func TestGroups_NilSafe(t *testing.T) {
	// Validation calls on a nil *Groups must not panic — represents
	// "no groups.yaml loaded".
	var g *Groups
	if g.Contains("x") {
		t.Error("nil Groups Contains should be false")
	}
	if _, ok := g.Recipients("x"); ok {
		t.Error("nil Groups Recipients should be (_, false)")
	}
}
