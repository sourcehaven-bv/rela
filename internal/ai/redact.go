package ai

import (
	"encoding/json"
	"strings"
)

// redactKey returns s with all occurrences of key (and "Bearer <key>")
// replaced with "<REDACTED>". When key is empty, s is returned unchanged.
//
// This is a defense-in-depth helper used at error and log construction
// sites that *could* see the API key. The key should never be passed
// into error messages in the first place; this is the safety net.
func redactKey(s, key string) string {
	if key == "" {
		return s
	}
	out := s
	// Order matters: replace the longer "Bearer <key>" form first so
	// the literal substring is fully removed before we touch the key
	// itself.
	out = strings.ReplaceAll(out, "Bearer "+key, "<REDACTED>")
	out = strings.ReplaceAll(out, key, "<REDACTED>")
	return out
}

// jsonUnmarshal is a thin wrapper that exists so errors.go can call into
// encoding/json without an import cycle when we eventually move helpers
// around. Today it is just json.Unmarshal.
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
