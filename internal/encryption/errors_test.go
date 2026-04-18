package encryption

import (
	"errors"
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
