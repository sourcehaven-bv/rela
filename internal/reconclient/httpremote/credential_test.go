package httpremote

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
