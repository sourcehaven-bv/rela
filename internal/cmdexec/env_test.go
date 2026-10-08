package cmdexec

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestWithoutWithheld(t *testing.T) {
	in := []string{
		"PATH=/bin", "RELA_TOKEN_KEY=k", "RELA_DATABASE_URL=postgres://u:p@h/db",
		"rela_token_key=k", "RELA_TOKEN_KEY_HINT=keep", "EMPTY=",
	}
	got := withoutWithheld(in)
	want := []string{"PATH=/bin", "RELA_TOKEN_KEY_HINT=keep", "EMPTY="}
	if !slices.Equal(got, want) {
		t.Fatalf("withoutWithheld = %q, want %q", got, want)
	}
}

// A configured command never sees the token key or the database DSN.
func TestRun_WithholdsSecretEnv(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("RELA_TOKEN_KEY", "token-key-value")
	t.Setenv("RELA_DATABASE_URL", "postgres://u:dsn-password@h/db")
	t.Setenv("CMDEXEC_VISIBLE", "visible-value")
	r := newRunner(t)
	out, _, err := r.Run(context.Background(), []string{"env"}, nil, true)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	s := string(out)
	for _, secret := range []string{"token-key-value", "dsn-password"} {
		if strings.Contains(s, secret) {
			t.Errorf("child environment holds %q", secret)
		}
	}
	if !strings.Contains(s, "CMDEXEC_VISIBLE=visible-value") {
		t.Error("child environment lost an ordinary variable")
	}
}
