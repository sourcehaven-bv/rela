package appbuild

import "testing"

// The DSN is consumed only by the postgres recipe, so on the FS and memory
// builds an end-to-end assertion cannot tell "option honored" from "option
// ignored" — a test written against Discover alone passes even when the option
// is dropped on the floor. (RES-S8CH9C records the same trap for the advisory
// lock keys: the obvious regression test passed against deliberately broken
// code.) These assert the resolution rule directly instead, with getenv
// injected so precedence is observable without mutating process environment.
func TestResolveDatabaseURL(t *testing.T) {
	const (
		fromOption = "postgres://option/db"
		fromEnv    = "postgres://environment/db"
	)

	env := func(v string) func(string) string {
		return func(key string) string {
			if key != "RELA_DATABASE_URL" {
				return ""
			}
			return v
		}
	}

	for _, tc := range []struct {
		name   string
		opts   []Option
		getenv func(string) string
		want   string
	}{
		{
			// The property the seam exists for: a caller that knows where the
			// data lives is not overridden by process-global state.
			name:   "explicit option beats a set environment",
			opts:   []Option{WithDatabaseURL(fromOption)},
			getenv: env(fromEnv),
			want:   fromOption,
		},
		{
			// Every existing call site passes no option and must keep working.
			name:   "no option falls back to the environment",
			getenv: env(fromEnv),
			want:   fromEnv,
		},
		{
			name:   "option applies when the environment is unset",
			opts:   []Option{WithDatabaseURL(fromOption)},
			getenv: env(""),
			want:   fromOption,
		},
		{
			name:   "neither source set yields empty",
			getenv: env(""),
			want:   "",
		},
		{
			// An empty option must not shadow a usable environment value —
			// otherwise WithDatabaseURL(someUnsetVar) would silently break a
			// deployment that relies on the env.
			name:   "empty option does not suppress the environment",
			opts:   []Option{WithDatabaseURL("")},
			getenv: env(fromEnv),
			want:   fromEnv,
		},
		{
			name:   "last option wins",
			opts:   []Option{WithDatabaseURL("postgres://first/db"), WithDatabaseURL(fromOption)},
			getenv: env(fromEnv),
			want:   fromOption,
		},
		{
			// WithDatabaseURL must not disturb unrelated options.
			name:   "coexists with other options",
			opts:   []Option{WithACL(nil), WithDatabaseURL(fromOption)},
			getenv: env(fromEnv),
			want:   fromOption,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDatabaseURL(tc.opts, tc.getenv); got != tc.want {
				t.Errorf("resolveDatabaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestResolveDatabaseURL_ReadsOnlyItsOwnVariable guards against a future
// refactor widening the ambient surface this ticket set out to narrow.
func TestResolveDatabaseURL_ReadsOnlyItsOwnVariable(t *testing.T) {
	var asked []string
	getenv := func(key string) string {
		asked = append(asked, key)
		return ""
	}

	resolveDatabaseURL(nil, getenv)

	if len(asked) != 1 || asked[0] != "RELA_DATABASE_URL" {
		t.Errorf("read environment %v, want exactly [RELA_DATABASE_URL]", asked)
	}
}
