package tokenstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// KeySecret is the global secrets.yaml key holding the encryption key. It
// equals secrets.TokenKey; it is repeated here because this package may
// depend only on state, and a test asserts the two agree.
const KeySecret = "token_key"

// KeyEnv is the environment variable read when secrets.yaml has no
// [KeySecret]. It suits container and systemd deployments.
const KeyEnv = "RELA_TOKEN_KEY"

// ErrNoKey reports that neither secrets.yaml nor the environment holds a
// token key.
var ErrNoKey = errors.New("tokenstore: no token_key: generate one with `openssl rand -base64 32` " +
	"and add it to .rela/secrets.yaml as token_key, or set " + KeyEnv)

// LoadKey returns the encryption key from the global secrets, else from the
// environment. secrets.yaml wins when both are set, like the SMTP password.
//
// Nil: global and getenv are accepted; a nil map has no key, and a nil
// getenv reads no environment.
func LoadKey(global map[string]string, getenv func(string) string) ([]byte, error) {
	raw := global[KeySecret]
	source := "secrets.yaml token_key"
	if raw == "" && getenv != nil {
		raw = getenv(KeyEnv)
		source = KeyEnv
	}
	if raw == "" {
		return nil, ErrNoKey
	}
	key, err := ParseKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	return key, nil
}

// ParseKey decodes a base64 key and checks its length.
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err != nil {
		return nil, errors.New("token key is not base64; generate one with `openssl rand -base64 32`")
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("token key must decode to %d bytes, got %d; "+
			"generate one with `openssl rand -base64 32`", KeySize, len(key))
	}
	return key, nil
}
