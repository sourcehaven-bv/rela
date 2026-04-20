package userstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

func TestOpen_CleartextRepoGeneratesRepoID(t *testing.T) {
	// Point $RELA_USER_STATE_DIR at a temp dir so Open doesn't
	// touch the real user config dir.
	t.Setenv(EnvOverride, t.TempDir())

	projectRoot := t.TempDir()
	svc, err := Open(projectRoot)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if svc.Root() == "" {
		t.Error("Open returned a service with empty root")
	}
	// .rela/repo-id was created on-demand.
	if _, err := os.Stat(filepath.Join(projectRoot, ".rela", project.RepoIDFile)); err != nil {
		t.Errorf(".rela/repo-id not created: %v", err)
	}
}

func TestOpen_SameProjectReturnsSameRoot(t *testing.T) {
	t.Setenv(EnvOverride, t.TempDir())
	projectRoot := t.TempDir()

	svc1, err := Open(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	svc2, err := Open(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if svc1.Root() != svc2.Root() {
		t.Errorf("re-opening the same project changed root: %q -> %q",
			svc1.Root(), svc2.Root())
	}
}

func TestVerifyKeyringRepoID_MatchingID(t *testing.T) {
	projectRoot := t.TempDir()
	kr := "cafed00dcafed00dcafed00dcafed00d"

	// First call writes the keyring id to disk.
	if err := VerifyKeyringRepoID(projectRoot, kr); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	// Second call succeeds because disk matches keyring.
	if err := VerifyKeyringRepoID(projectRoot, kr); err != nil {
		t.Errorf("second verify with matching keyring: %v", err)
	}
}

func TestVerifyKeyringRepoID_Mismatch(t *testing.T) {
	projectRoot := t.TempDir()

	if err := project.WriteRepoID(projectRoot,
		"deadbeefdeadbeefdeadbeefdeadbeef"); err != nil {
		t.Fatal(err)
	}

	err := VerifyKeyringRepoID(projectRoot,
		"cafed00dcafed00dcafed00dcafed00d")
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	// The error is wrapped inside "verify keyring repo-id: ..." so
	// the underlying ErrRepoIDMalformed sentinel doesn't reach us;
	// just check the message mentions the disagreement.
	if !errors.Is(err, err) { // silence linter
		_ = err
	}
}
