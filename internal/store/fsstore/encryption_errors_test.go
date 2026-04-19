package fsstore

import (
	"errors"
	"strings"
	"testing"
)

func TestEncryptionError_Error(t *testing.T) {
	cases := []struct {
		name    string
		err     *EncryptionError
		wantSub []string
	}{
		{
			name:    "file-level",
			err:     &EncryptionError{Kind: ErrKindMissingKeyring},
			wantSub: []string{"fsstore: encryption", "missing_keyring"},
		},
		{
			name:    "with property",
			err:     &EncryptionError{Kind: ErrKindOpaqueWrite, Property: "description"},
			wantSub: []string{"opaque_write", "property description"},
		},
		{
			name:    "with cause",
			err:     &EncryptionError{Kind: ErrKindCorruptedFile, Cause: errors.New("inner reason")},
			wantSub: []string{"corrupted_file", "inner reason"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.err.Error()
			for _, sub := range tc.wantSub {
				if !strings.Contains(msg, sub) {
					t.Errorf("Error() = %q, want to contain %q", msg, sub)
				}
			}
		})
	}
}

func TestEncryptionError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	e := &EncryptionError{Kind: ErrKindCorruptedFile, Cause: inner}
	if !errors.Is(e, inner) {
		t.Error("errors.Is(e, inner) = false, want true")
	}
}

func TestEncryptionError_UnwrapNil(t *testing.T) {
	// An error without a Cause still unwraps cleanly (to nil).
	e := &EncryptionError{Kind: ErrKindMissingKeyring}
	if got := errors.Unwrap(e); got != nil {
		t.Errorf("Unwrap = %v, want nil", got)
	}
}
