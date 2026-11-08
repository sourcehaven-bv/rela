package cli

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/peerkey"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// PeerCmd groups the commands for sync peers (FEAT-CMVZQFI).
type PeerCmd struct {
	Key  PeerKeyCmd  `cmd:"" help:"Manage the keys sync peers authenticate with."`
	Pull PeerPullCmd `cmd:"" help:"Pull the rows a peer in peers.yaml maps into this project."`
}

// PeerKeyCmd groups the peer key commands.
type PeerKeyCmd struct {
	Create PeerKeyCreateCmd `cmd:"" help:"Generate a peer key and print the secrets.yaml line for the source."`
}

// PeerKeyCreateCmd generates a peer key (DEC-81161C3).
//
// The key goes to stdout and nowhere else, so it can be redirected straight
// into the peer's secret store. The line the source operator adds to
// `.rela/secrets.yaml` holds only the digest and goes to stderr. Nothing is
// written to disk: the key belongs on the peer, the digest on the source, and
// the two are often different machines.
type PeerKeyCreateCmd struct {
	Name string `arg:"" help:"Peer name: 1-63 lowercase letters, digits, '_' or '-'. The peer authenticates as peer:<name>."`
}

// Run generates the key.
func (c *PeerKeyCreateCmd) Run() error {
	return c.run(rand.Reader, os.Stdout, os.Stderr)
}

func (c *PeerKeyCreateCmd) run(random io.Reader, stdout, stderr io.Writer) error {
	if !principal.ValidPeerName(c.Name) {
		return fmt.Errorf("invalid peer name %q: use 1-63 lowercase letters, digits, '_' or '-', "+
			"starting with a letter or digit", c.Name)
	}
	key, digest, err := peerkey.Generate(random)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintln(stdout, key); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stderr, "Add this line to .rela/%[1]s on the source, or to its systemd credential if it "+
		"has one (the credential takes precedence over the file):\n\n"+
		"%[2]s%[3]s: %[4]s\n\n"+
		"Then grant %[5]s%[3]s a read role in the source's acl.yaml. Give the key above to the peer, which "+
		"stores it as a secret such as %[6]s<source> and passes it with --token-secret.\n",
		secrets.ConfigFile, secrets.PeerKeyPrefix, c.Name, digest, principal.PeerPrefix, secrets.PeerCredentialPrefix)
	return err
}
