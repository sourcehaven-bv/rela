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
// from <projectRoot>/keys. The private key is resolved in this order:
//
//  1. $RELA_KEY_FILE (if set; missing file is an error)
//  2. <projectRoot>/.rela/key (if present)
//  3. ~/.config/rela/key (if present)
//
// A missing private key is not an error — Keyring.Unwrap returns
// ErrNoPrivateKey at call time. This lets read-only flows work without
// a configured key.
func LoadFromDir(projectRoot string) (*Keyring, error) {
	keysDir := filepath.Join(projectRoot, projectKeysDir)
	relaDir := filepath.Join(projectRoot, projectRelaDir)
	privPath, err := resolvePrivateKeyPath(relaDir, userHomeDir)
	if err != nil {
		return nil, err
	}
	return LoadKeyring(keysDir, privPath)
}

// resolvePrivateKeyPath walks the private-key precedence chain and
// returns the first path that resolves to an existing file, or "" if
// none is configured. If $RELA_KEY_FILE is set but the file does not
// exist, an error is returned.
func resolvePrivateKeyPath(relaDir string, home func() (string, error)) (string, error) {
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
		return "", nil //nolint:nilerr // no home dir → treat as "no private key configured"
	}
	userPath := filepath.Join(h, ".config", userConfigSubdir, projectKeyFile)
	if _, err := os.Stat(userPath); err == nil {
		return userPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("encryption: stat %s: %w", userPath, err)
	}
	return "", nil
}

func userHomeDir() (string, error) {
	return os.UserHomeDir()
}
