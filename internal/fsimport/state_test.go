package fsimport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AC7: a temp file left under .rela/ by an interrupted write is never
// copied into the target's state KV, even below a state prefix.
func TestClassifyRela_TempFiles(t *testing.T) {
	tests := []struct {
		path string
		want relaClass
	}{
		{"theme/logo.png.0123456789abcdef.tmp", relaTemp},
		{"migration/state.json.tmp", relaTemp},
		{"user-defaults.yaml.new", relaTemp},
		{"palette.yaml.0123456789abcdef.tmp", relaTemp},
		{"theme/logo.png", relaState},
		{"palette.yaml", relaState},
		{"config.yaml", relaConfig},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got, _ := classifyRela(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}
