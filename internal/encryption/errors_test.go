package encryption

import (
	"errors"
	"strings"
	"testing"
)

func TestSentinels_ErrorsIs(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"errBadPEM filename", errBadPEM("alice.pub", errors.New("x")), ErrBadPEM},
		{"errBadPEM no filename", errBadPEM("", errors.New("x")), ErrBadPEM},
		{"errBadPEMType", errBadPEMType("FOO", "BAR"), ErrBadPEM},
		{"errBadPEMLength", errBadPEMLength(1, 2), ErrBadPEM},
		{"errBadBlobMagic", errBadBlobMagic(), ErrBadBlob},
		{"errBadBlobVersion", errBadBlobVersion(0x02), ErrBadBlob},
		{"errBadBlobLength", errBadBlobLength(0), ErrBadBlob},
		{"errDecryptGCM", errDecryptGCM(errors.New("gcm")), ErrDecrypt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.want) {
				t.Fatalf("errors.Is(%v, %v) = false", tc.err, tc.want)
			}
		})
	}
}

// TestErrDecryptGCM_DoesNotWrapCause protects against a future
// refactor adding fmt.Errorf("%w: %v", ErrDecrypt, cause) — which
// would leak the underlying GCM cause into error messages and
// potentially create an oracle for attackers.
func TestErrDecryptGCM_DoesNotWrapCause(t *testing.T) {
	cause := errors.New("AUTH_TAG_MISMATCH_XYZ")
	got := errDecryptGCM(cause)
	if errors.Is(got, cause) {
		t.Fatal("errDecryptGCM must not wrap the cause via %w")
	}
	if strings.Contains(got.Error(), "AUTH_TAG_MISMATCH_XYZ") {
		t.Fatalf("errDecryptGCM must not embed the cause string: %q", got.Error())
	}
}
