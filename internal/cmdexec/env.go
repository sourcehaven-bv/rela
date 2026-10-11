package cmdexec

import (
	"os"
	"strings"
)

// withheldEnv names the environment variables no child process inherits:
// the key that seals connector tokens (TKT-01KZSO) and the database DSN,
// which carries the database password. Both reach rela through the
// environment only, so leaving them out here keeps them out of every
// configured command.
var withheldEnv = []string{"RELA_TOKEN_KEY", "RELA_DATABASE_URL"}

// Environ returns the process environment without the variables in
// withheldEnv. Every command rela starts gets its environment from here.
func Environ() []string {
	return withoutWithheld(os.Environ())
}

func withoutWithheld(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !isWithheld(name) {
			out = append(out, kv)
		}
	}
	return out
}

// isWithheld compares without case, because Windows reads environment
// names that way.
func isWithheld(name string) bool {
	for _, w := range withheldEnv {
		if strings.EqualFold(name, w) {
			return true
		}
	}
	return false
}
