package cli

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// SecretsCmd groups the secrets helpers.
type SecretsCmd struct {
	CredentialName SecretsCredentialNameCmd `cmd:"" name:"credential-name" help:"Print the systemd credential name for this project."`
}

// SecretsCredentialNameCmd prints the systemd credential name for the project.
//
// The name embeds a hash of the project's absolute path so two projects sharing
// a directory name cannot resolve to one credential, which makes it something
// an operator cannot derive by hand — hence this command. It writes the bare
// name so it can be substituted straight into a unit file.
type SecretsCredentialNameCmd struct{}

// Run dispatches `rela secrets credential-name`.
func (c *SecretsCredentialNameCmd) Run(svc *readServices) error {
	name := secrets.CredentialName(svc.Paths.CacheDir)
	if name == "" {
		return fmt.Errorf("secrets: cannot derive a credential name for %q", svc.Paths.CacheDir)
	}
	fmt.Println(name)
	return nil
}

// globalSecret reads name from the global section of the project's secrets.
// A per-script override is for that script and never applies here. The error
// names the secret, never a value.
func globalSecret(relaDir, name string) (string, error) {
	v, ok, err := secrets.Global(relaDir, name)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("no secret %q in the global section", name)
	}
	return v, nil
}
