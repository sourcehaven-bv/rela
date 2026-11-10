package tokenstore

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

func TestRemoteCredentialPrefix_AgreesWithSecrets(t *testing.T) {
	if remoteCredentialPrefix != secrets.RemoteCredentialPrefix {
		t.Fatalf("remoteCredentialPrefix %q != secrets.RemoteCredentialPrefix %q",
			remoteCredentialPrefix, secrets.RemoteCredentialPrefix)
	}
}
