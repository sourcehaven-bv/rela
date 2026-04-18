package encryption

import (
	"errors"
	"fmt"
)

var (
	ErrNoPrivateKey = errors.New("encryption: no private key configured")
	ErrBadPEM       = errors.New("encryption: malformed PEM")
	ErrBadBlob      = errors.New("encryption: malformed wrapped blob")
	ErrDecrypt      = errors.New("encryption: decryption failed")
)

func errBadPEM(filename string, cause error) error {
	if filename == "" {
		return fmt.Errorf("%w: %s", ErrBadPEM, cause.Error())
	}
	return fmt.Errorf("%w: %s: %s", ErrBadPEM, filename, cause.Error())
}

func errBadPEMType(gotType, wantType string) error {
	return fmt.Errorf("%w: block type %q, want %q", ErrBadPEM, gotType, wantType)
}

func errBadPEMLength(gotLen, wantLen int) error {
	return fmt.Errorf("%w: payload length %d, want %d", ErrBadPEM, gotLen, wantLen)
}

func errBadBlobMagic() error {
	return fmt.Errorf("%w: bad magic", ErrBadBlob)
}

func errBadBlobVersion(got byte) error {
	return fmt.Errorf("%w: version %d unsupported", ErrBadBlob, got)
}

func errBadBlobLength(got int) error {
	return fmt.Errorf("%w: length %d, want %d", ErrBadBlob, got, wrappedBlobSize)
}

func errDecryptGCM(cause error) error {
	_ = cause
	return ErrDecrypt
}
