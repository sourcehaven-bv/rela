package userstate

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Open resolves the repo-id at projectRoot (creating .rela/repo-id
// on first access) and returns an FSService rooted under the user
// config directory for that id.
//
// Encrypted repos call VerifyKeyringRepoID after loading the
// keyring so the on-disk .rela/repo-id can be cross-checked against
// the keyring's RepoID — a mismatch typically means a .rela/
// directory was copied in from another project.
func Open(projectRoot string) (FSService, error) {
	id, err := project.ResolveRepoID(projectRoot, "")
	if err != nil {
		return nil, err
	}
	return NewFSWithRepoID(projectRoot, id)
}

// VerifyKeyringRepoID checks that .rela/repo-id at projectRoot
// matches keyringRepoID (the id embedded in recipients.age). Called
// after keyring load on encrypted repos. If .rela/repo-id is
// missing (first access on a repo that was encrypted elsewhere),
// the keyring id is written to disk. If present and different,
// returns a wrapped error — almost certainly a copied-in .rela/.
func VerifyKeyringRepoID(projectRoot, keyringRepoID string) error {
	if _, err := project.ResolveRepoID(projectRoot, keyringRepoID); err != nil {
		return fmt.Errorf("userstate: verify keyring repo-id: %w", err)
	}
	return nil
}
