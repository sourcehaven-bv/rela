package encryption

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	envKeyFile       = "RELA_KEY_FILE"
	projectRelaDir   = ".rela"
	projectKeyFile   = "key"
	userConfigSubdir = "rela"

	// ConfigFileName is the filename under .rela/ whose presence
	// flips a rela project into encryption-enabled mode.
	// Historically this marker doubled as the recipient list; in
	// the S2 design, recipients live in <root>/recipients.age
	// (authoritative and encrypted) and this file is a simple
	// presence marker kept for back-compat with tooling that only
	// peeks at .rela/ to decide whether a repo is encrypted.
	ConfigFileName = "encryption.yaml"
)

// IsEnabled reports whether encryption is enabled for the project at
// projectRoot — true iff <projectRoot>/recipients.age exists on
// disk. A stat error other than ErrNotExist surfaces as false +
// error so callers can distinguish a misconfigured filesystem from
// a genuinely cleartext repo.
//
// Uses the presence of recipients.age (the S2 authoritative file)
// rather than .rela/encryption.yaml so adversary-deletion of the
// marker can't mask an encrypted repo as cleartext.
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

// LoadFromDir loads a Keyring rooted at projectRoot. The local
// identity is resolved in this order:
//
//  1. $RELA_KEY_FILE (if set; missing file is an error)
//  2. <projectRoot>/.rela/key (if present)
//  3. ~/.config/rela/key (if present)
//
// Then <projectRoot>/recipients.age is decrypted with that identity
// to produce the authoritative recipient list.
//
// Returns os.ErrNotExist when recipients.age is absent (repo is
// cleartext) so callers can distinguish that from a load failure.
// Returns ErrNoPrivateKey when the repo is encrypted but no
// identity could be resolved.
func LoadFromDir(projectRoot string) (*Keyring, error) {
	relaDir := filepath.Join(projectRoot, projectRelaDir)
	identityPath, err := resolveIdentityPath(relaDir, userHomeDir)
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
func resolveIdentityPath(relaDir string, home func() (string, error)) (string, error) {
	if env := os.Getenv(envKeyFile); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("encryption: %s=%q: %w", envKeyFile, env, err)
		}
		return env, nil
	}
	projectPath := filepath.Join(relaDir, projectKeyFile)
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("encryption: stat %s: %w", projectPath, err)
	}
	h, err := home()
	if err != nil {
		return "", nil //nolint:nilerr // no home dir -> treat as "no identity configured"
	}
	userPath := filepath.Join(h, ".config", userConfigSubdir, projectKeyFile)
	if _, err := os.Stat(userPath); err == nil {
		return userPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("encryption: stat %s: %w", userPath, err)
	}
	return "", nil
}

func userHomeDir() (string, error) { return os.UserHomeDir() }
