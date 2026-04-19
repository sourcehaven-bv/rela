// keygen is a throwaway helper: generate a keypair and write
// <outdir>/<name>.pub and <outdir>/<name>.key. Used to bootstrap
// the encryption demo before slice-5 ships proper `rela keys`
// commands. Delete this package once `rela keys generate` lands.
package main

import (
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: keygen <name> <outdir>")
		os.Exit(1)
	}
	name, outdir := os.Args[1], os.Args[2]

	kp, err := encryption.GenerateKeypair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	priv, err := encryption.MarshalPrivateKeyPEM(kp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal priv: %v\n", err)
		os.Exit(1)
	}
	pub, err := encryption.MarshalPublicKeyPEM(kp.PublicKey())
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal pub: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outdir, 0o700); err != nil { //nolint:mnd // standard user dir perms
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outdir+"/"+name+".key", priv, 0o600); err != nil { //nolint:mnd // user-only read perms for private key
		fmt.Fprintf(os.Stderr, "write priv: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outdir+"/"+name+".pub", pub, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write pub: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s/%s.key and %s/%s.pub\n", outdir, name, outdir, name)
}
