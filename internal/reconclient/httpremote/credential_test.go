package httpremote

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

func TestCredential_Read(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "token")
	if err := os.WriteFile(file, []byte(" from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(dir, "big")
	if err := os.WriteFile(big, bytes.Repeat([]byte("a"), MaxCredentialFile+1), 0o600); err != nil {
		t.Fatal(err)
	}
	getenv := func(k string) string { return map[string]string{"TOK": "from-env", "EMPTY": " "}[k] }
	secret := func(name string) (string, error) {
		switch name {
		case "remote_credential.up":
			return "from-secret", nil
		case "remote_credential.broken":
			return "", errors.New("store down")
		}
		return "", nil
	}
	cases := []struct {
		name    string
		c       Credential
		want    string
		wantErr string
	}{
		{"none", Credential{}, "", ""},
		{"env", Credential{Env: "TOK"}, "from-env", ""},
		{"file", Credential{File: file}, "from-file", ""},
		{"secret", Credential{Secret: "remote_credential.up"}, "from-secret", ""},
		{"empty env", Credential{Env: "EMPTY"}, "", "no credential"},
		{"unset secret", Credential{Secret: "remote_credential.none"}, "", "no credential"},
		{"secret error", Credential{Secret: "remote_credential.broken"}, "", "store down"},
		{"script readable name", Credential{Secret: "api_key"}, "", "must start with"},
		{"bare prefix", Credential{Secret: "remote_credential."}, "", "must start with"},
		{"token key", Credential{Secret: "token_key"}, "", "must start with"},
		{"two sources", Credential{Env: "TOK", File: file}, "", "name one credential source"},
		{"missing file", Credential{File: filepath.Join(dir, "nope")}, "", "no such file"},
		{"big file", Credential{File: big}, "", "larger than"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.c.Read(getenv, secret)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("Read = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := (Credential{Secret: "remote_credential.up"}).Read(nil, nil); err == nil {
		t.Fatal("a secret without a secret reader must fail")
	}
	if _, err := (Credential{Env: "TOK"}).Read(nil, nil); err == nil {
		t.Fatal("an env source without getenv must fail")
	}
}

func TestCredential_ValidateDeclared(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "remote_credential.upstream")
	tests := []struct {
		name    string
		c       Credential
		wantErr string
	}{
		{name: "none", c: Credential{}},
		{name: "secret", c: Credential{Secret: "remote_credential.upstream"}},
		{name: "env", c: Credential{Env: "REMOTE_CREDENTIAL_UPSTREAM"}},
		{name: "file", c: Credential{File: abs}},
		{name: "two sources", c: Credential{Env: "REMOTE_CREDENTIAL_A", File: abs}, wantErr: "name one credential source"},
		{name: "env without prefix", c: Credential{Env: "HOME"}, wantErr: "must start with"},
		{name: "env prefix only", c: Credential{Env: CredentialEnvPrefix}, wantErr: "must start with"},
		{name: "relative file", c: Credential{File: "remote_credential.upstream"}, wantErr: "absolute"},
		{
			name:    "file with another name",
			c:       Credential{File: filepath.Join(t.TempDir(), "id_rsa")},
			wantErr: "file name must start with",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.ValidateDeclared()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDeclared() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateDeclared() = %v, want an error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestGlobalSecrets(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		load    func() (map[string]string, error)
		want    string
		wantErr error
		anyErr  bool
	}{
		{name: "nil load", load: nil, anyErr: true},
		{name: "found", load: func() (map[string]string, error) {
			return map[string]string{"remote_credential.a": "tok"}, nil
		}, want: "tok"},
		{name: "no secrets file", load: func() (map[string]string, error) {
			return nil, secrets.ErrNotFound
		}},
		{name: "load error", load: func() (map[string]string, error) { return nil, boom }, wantErr: boom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GlobalSecrets(tc.load)("remote_credential.a")
			switch {
			case tc.anyErr:
				if err == nil {
					t.Fatal("want an error")
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
			case err != nil:
				t.Fatalf("err = %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
