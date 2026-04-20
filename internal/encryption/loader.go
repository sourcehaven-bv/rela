package encryption

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

const (
	envKeyFile = "RELA_KEY_FILE"

	// ConfigFileName is the filename under .rela/ whose presence
	// historically flipped a rela project into encryption-enabled
	// mode. The S2 design promoted <root>/recipients.age to the
	// authoritative signal; this file is retained only for tooling
	// that still peeks at .rela/ to decide whether a repo is
	// encrypted.
	ConfigFileName = "encryption.yaml"

	// identityKey names the user-state key holding the age
	// private-key identity. Matches the key used by `rela keys init`.
	identityKey = "key"
)

// IsEnabled reports whether encryption is enabled for the project at
// projectRoot — true iff <projectRoot>/recipients.age exists on
// disk. A stat error other than ErrNotExist surfaces as false +
// error so callers can distinguish a misconfigured filesystem from
// a genuinely cleartext repo.
func IsEnabled(projectRoot string) (bool, error) {
	recipientsPath := filepath.Join(projectRoot, RecipientsFileName)
	_, err := os.Stat(recipientsPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("encryption: stat %s: %w", recipientsPath, err)
}

// LoadFromDir loads a Keyring rooted at projectRoot, reading the
// local age identity via the user-state service.
//
// Identity resolution precedence:
//
//  1. $RELA_KEY_FILE (if set; missing file is an error)
//  2. us.Path(identityKey) in the user-state directory
//
// There is no fallback to the project tree; an age identity
// checked into the repo would defeat the at-rest encryption
// threat model (cloud-synced repo = cloud-synced key).
//
// Returns os.ErrNotExist when recipients.age is absent (repo is
// cleartext) so callers can distinguish that from a load failure.
// Returns ErrNoPrivateKey when the repo is encrypted but no
// identity could be resolved.
func LoadFromDir(projectRoot string, us userstate.FSService) (*Keyring, error) {
	if us == nil {
		return nil, errors.New("encryption: LoadFromDir: nil user-state service")
	}
	identityPath, err := resolveIdentityPath(us)
	if err != nil {
		return nil, err
	}
	if identityPath == "" {
		return nil, ErrNoPrivateKey
	}
	f, err := os.Open(identityPath)
	if err != nil {
		return nil, fmt.Errorf("encryption: open identity %s: %w", identityPath, err)
	}
	defer f.Close()
	id, err := ReadIdentity(f)
	if err != nil {
		return nil, fmt.Errorf("encryption: %s: %w", filepath.Base(identityPath), err)
	}
	return LoadKeyring(filepath.Join(projectRoot, RecipientsFileName), id)
}

// resolveIdentityPath walks the identity-path precedence chain and
// returns the first path that resolves to an existing file, or "" if
// none is configured. If $RELA_KEY_FILE is set but the target file
// does not exist, an error is returned (explicit overrides MUST
// resolve).
func resolveIdentityPath(us userstate.FSService) (string, error) {
	if env := os.Getenv(envKeyFile); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("encryption: %s=%q: %w", envKeyFile, env, err)
		}
		return env, nil
	}
	userPath := us.Path(identityKey)
	if _, err := os.Stat(userPath); err == nil {
		return userPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("encryption: stat %s: %w", userPath, err)
	}
	return "", nil
}
