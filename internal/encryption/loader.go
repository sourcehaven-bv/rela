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
	projectKeysDir   = "keys"
	userConfigSubdir = "rela"
)

// LoadFromDir loads a Keyring rooted at projectRoot. Recipients come
// from <projectRoot>/keys. The local identity is resolved in this
// order:
//
//  1. $RELA_KEY_FILE (if set; missing file is an error)
//  2. <projectRoot>/.rela/key (if present)
//  3. ~/.config/rela/key (if present)
//
// A missing identity file in positions 2 and 3 is fine; Unseal will
// return ErrNoPrivateKey at call time. This lets read-only flows
// inspect an unencrypted repo without a configured key.
func LoadFromDir(projectRoot string) (*Keyring, error) {
	keysDir := filepath.Join(projectRoot, projectKeysDir)
	relaDir := filepath.Join(projectRoot, projectRelaDir)
	identityPath, err := resolveIdentityPath(relaDir, userHomeDir)
	if err != nil {
		return nil, err
	}
	return LoadKeyring(keysDir, identityPath)
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
