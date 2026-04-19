package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/encryption"
)

// keyFilePerm is the filesystem permission for private-key files
// (readable only by the owner).
const keyFilePerm os.FileMode = 0o600

// keysCmd is the top-level parent for all encryption-related
// commands. Its subcommands manage the recipient keyring and the
// cleartext/encrypted state of the repository.
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage at-rest encryption keys and recipients",
	Long: `Manage age-based at-rest encryption for this rela project.

A repo is encryption-enabled when .rela/encryption.yaml is present.
Public keys live in <repo>/keys/*.pub; private keys live outside the
project (see $RELA_KEY_FILE, .rela/key, ~/.config/rela/key in order).

Subcommands:
  generate    Generate a fresh age identity (pub+priv)
  init        Enable encryption on this repo (seals every data file)
  decrypt     Disable encryption on this repo (unseals every data file)
  add         Add a recipient public key and re-encrypt all files
  remove      Remove a recipient and re-encrypt all files
  status      Show encryption status and recipient list`,
}

// Flags specific to subcommands.
var (
	keysGenerateOut   string
	keysAddPub        string
	keysInitPub       string
	keysInitRecipient string
	keysInitIdentity  string
)

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysInitCmd)
	keysCmd.AddCommand(keysDecryptCmd)
	keysCmd.AddCommand(keysAddCmd)
	keysCmd.AddCommand(keysRemoveCmd)
	keysCmd.AddCommand(keysStatusCmd)

	keysGenerateCmd.Flags().StringVar(&keysGenerateOut, "out", "",
		"directory to write <name>.pub and <name>.key (required)")

	keysInitCmd.Flags().StringVar(&keysInitRecipient, "recipient", "",
		"name of the first recipient (also the pub-file stem: <name>.pub)")
	keysInitCmd.Flags().StringVar(&keysInitPub, "pub", "",
		"age public key string for the first recipient (e.g. age1...)")
	keysInitCmd.Flags().StringVar(&keysInitIdentity, "identity", "",
		"path to the private identity file to install at .rela/key")

	keysAddCmd.Flags().StringVar(&keysAddPub, "pub", "",
		"age public key string for the new recipient")
}

// --- generate ---

var keysGenerateCmd = &cobra.Command{
	Use:         "generate <name>",
	Short:       "Generate a fresh age identity",
	Long:        `Generate an age keypair and write <out>/<name>.pub and <out>/<name>.key.`,
	Annotations: map[string]string{skipProjectDiscovery: "true"},
	Args:        cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		if keysGenerateOut == "" {
			return errors.New("--out is required")
		}
		if err := os.MkdirAll(keysGenerateOut, 0o755); err != nil {
			return err
		}
		id, err := encryption.GenerateIdentity()
		if err != nil {
			return err
		}
		pubPath := filepath.Join(keysGenerateOut, name+".pub")
		keyPath := filepath.Join(keysGenerateOut, name+".key")
		if err := os.WriteFile(pubPath, []byte(id.PublicRecipient().String()+"\n"), 0o644); err != nil {
			return err
		}
		// Private key is sensitive; chmod 0600.
		if err := os.WriteFile(keyPath, []byte(encryption.IdentitySecretForTest(id)+"\n"), keyFilePerm); err != nil {
			return err
		}
		out.WriteSuccess("Generated age identity %q", name)
		out.WriteMessage("  public:  %s", pubPath)
		out.WriteMessage("  private: %s (DO NOT commit)", keyPath)
		return nil
	},
}

// --- init ---

var keysInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Enable at-rest encryption for this project",
	Long: `Encrypt every entity, relation, and attachment in this project.

Usage:
  rela keys init --recipient <name> --pub <age1...> [--identity <path>]

--recipient must be a filename-stem (alphanumerics + hyphen/underscore).
--pub is the age public key string for that recipient.
--identity, if set, copies the private key to .rela/key so this user
  becomes the default reader of the encrypted repo.

The command refuses to proceed if the repo is already encrypted.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := validateRecipientName(keysInitRecipient); err != nil {
			return err
		}
		if keysInitPub == "" {
			return errors.New("--pub is required")
		}
		rec, err := encryption.ParseRecipient(keysInitPub)
		if err != nil {
			return err
		}

		if err := ensureCleartextRepo(); err != nil {
			return err
		}

		// Write the recipient pubkey.
		keysDir := filepath.Join(projectCtx.Root, "keys")
		if err := os.MkdirAll(keysDir, 0o755); err != nil {
			return err
		}
		pubPath := filepath.Join(keysDir, keysInitRecipient+".pub")
		if err := os.WriteFile(pubPath, []byte(rec.String()+"\n"), 0o644); err != nil {
			return err
		}

		// Copy the private identity into .rela/key if provided.
		if keysInitIdentity != "" {
			if err := copyFile(keysInitIdentity, filepath.Join(projectCtx.CacheDir, "key"), keyFilePerm); err != nil {
				return err
			}
		}

		// Seal every data file under this new recipient set.
		if err := sealAllFiles(projectCtx.Root, []encryption.Recipient{rec}); err != nil {
			return err
		}

		// Write the encryption marker.
		if err := writeEncryptionConfig(projectCtx.CacheDir, []string{keysInitRecipient}); err != nil {
			return err
		}

		out.WriteSuccess("Encryption enabled. Repo is now sealed for %s.", keysInitRecipient)
		return nil
	},
}

// --- decrypt ---

var keysDecryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Disable at-rest encryption for this project",
	Long:  `Unseal every entity, relation, and attachment, then remove .rela/encryption.yaml.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := ensureEncryptedRepo(); err != nil {
			return err
		}

		// Load the keyring first (we need an identity to unseal).
		kr, err := encryption.LoadFromDir(projectCtx.Root)
		if err != nil {
			return err
		}
		if !kr.HasIdentity() {
			return errors.New("no local identity loaded; cannot unseal (set $RELA_KEY_FILE or place .rela/key)")
		}
		if kr.LocalName() == "" {
			return errors.New("loaded identity is not in the recipient list; cannot unseal")
		}

		// Unseal every data file.
		if err := unsealAllFiles(projectCtx.Root, kr); err != nil {
			return err
		}

		// Remove the encryption marker and the keys dir.
		if err := os.Remove(filepath.Join(projectCtx.CacheDir, app.EncryptionConfigFile)); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(projectCtx.Root, "keys")); err != nil {
			return err
		}

		out.WriteSuccess("Encryption disabled. Repo is now cleartext.")
		return nil
	},
}

// --- add ---

var keysAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a recipient and re-encrypt all files",
	Long:  `Add a new recipient public key and re-encrypt every data file so the recipient can read.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		if err := validateRecipientName(name); err != nil {
			return err
		}
		if keysAddPub == "" {
			return errors.New("--pub is required")
		}
		rec, err := encryption.ParseRecipient(keysAddPub)
		if err != nil {
			return err
		}
		if err = ensureEncryptedRepo(); err != nil {
			return err
		}
		keysDir := filepath.Join(projectCtx.Root, "keys")
		pubPath := filepath.Join(keysDir, name+".pub")
		if _, statErr := os.Stat(pubPath); statErr == nil {
			return fmt.Errorf("recipient %s already exists", name)
		}
		if err = os.WriteFile(pubPath, []byte(rec.String()+"\n"), 0o644); err != nil {
			return err
		}

		// Re-encrypt everything.
		kr, err := encryption.LoadFromDir(projectCtx.Root)
		if err != nil {
			return err
		}
		if !kr.HasIdentity() {
			return errors.New("no local identity loaded; cannot re-encrypt")
		}
		if err = reencryptAll(projectCtx.Root, kr); err != nil {
			return err
		}
		if err = writeEncryptionConfig(projectCtx.CacheDir, kr.RecipientNames()); err != nil {
			return err
		}
		out.WriteSuccess("Added recipient %q and re-encrypted %d data files.", name, len(kr.RecipientNames()))
		return nil
	},
}

// --- remove ---

var keysRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a recipient and re-encrypt all files",
	Long:  `Remove the named recipient and re-encrypt every data file under the remaining recipients.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		if err := ensureEncryptedRepo(); err != nil {
			return err
		}
		pubPath := filepath.Join(projectCtx.Root, "keys", name+".pub")
		if _, err := os.Stat(pubPath); err != nil {
			return fmt.Errorf("recipient %s not found", name)
		}
		if err := os.Remove(pubPath); err != nil {
			return err
		}
		kr, err := encryption.LoadFromDir(projectCtx.Root)
		if err != nil {
			return err
		}
		if len(kr.Recipients()) == 0 {
			return errors.New("refusing to remove last recipient; run `rela keys decrypt` instead")
		}
		if !kr.HasIdentity() {
			return errors.New("no local identity loaded; cannot re-encrypt")
		}
		if err := reencryptAll(projectCtx.Root, kr); err != nil {
			return err
		}
		if err := writeEncryptionConfig(projectCtx.CacheDir, kr.RecipientNames()); err != nil {
			return err
		}
		out.WriteSuccess("Removed recipient %q and re-encrypted %d data files.", name, len(kr.RecipientNames()))
		return nil
	},
}

// --- status ---

var keysStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show encryption status and recipient list",
	RunE: func(_ *cobra.Command, _ []string) error {
		encrypted, err := repoIsEncrypted()
		if err != nil {
			return err
		}
		if !encrypted {
			out.WriteMessage("Repo is cleartext (no .rela/encryption.yaml).")
			return nil
		}
		kr, err := encryption.LoadFromDir(projectCtx.Root)
		if err != nil {
			return err
		}
		out.WriteMessage("Repo is encrypted.")
		out.WriteMessage("Recipients (%d):", len(kr.RecipientNames()))
		for _, n := range kr.RecipientNames() {
			marker := ""
			if n == kr.LocalName() {
				marker = " (you)"
			}
			r, _ := kr.Recipient(n)
			out.WriteMessage("  %s%s  %s", n, marker, r.String())
		}
		if !kr.HasIdentity() {
			out.WriteMessage("No local identity loaded.")
		} else if kr.LocalName() == "" {
			out.WriteMessage("Local identity does not match any recipient.")
		}
		return nil
	},
}

// --- helpers ---

// validateRecipientName rejects names that would break the filename
// stem convention <name>.pub.
func validateRecipientName(name string) error {
	if name == "" {
		return errors.New("recipient name is required (--recipient)")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return fmt.Errorf("invalid recipient name %q (allowed: alphanumeric + hyphen/underscore)", name)
		}
	}
	return nil
}

func repoIsEncrypted() (bool, error) {
	_, err := os.Stat(filepath.Join(projectCtx.CacheDir, app.EncryptionConfigFile))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func ensureCleartextRepo() error {
	enc, err := repoIsEncrypted()
	if err != nil {
		return err
	}
	if enc {
		return errors.New("repo is already encrypted (see .rela/encryption.yaml)")
	}
	return nil
}

func ensureEncryptedRepo() error {
	enc, err := repoIsEncrypted()
	if err != nil {
		return err
	}
	if !enc {
		return errors.New("repo is not encrypted (run `rela keys init` first)")
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, perm)
}

// sealAllFiles walks the data dirs under root and seals every
// cleartext file for recipients. Used by `rela keys init` and
// exposed as a package-private function for testing.
func sealAllFiles(root string, recipients []encryption.Recipient) error {
	return walkDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if encryption.LooksSealed(raw) {
			return nil // already sealed, skip
		}
		sealed, err := encryption.Seal(raw, recipients)
		if err != nil {
			return err
		}
		return os.WriteFile(path, sealed, 0o644)
	})
}

// unsealAllFiles walks the same set and unseals every sealed file.
func unsealAllFiles(root string, kr *encryption.Keyring) error {
	return walkDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !encryption.LooksSealed(raw) {
			return nil
		}
		cleartext, err := encryption.Unseal(raw, kr.Identity())
		if err != nil {
			return err
		}
		return os.WriteFile(path, cleartext, 0o644)
	})
}

// reencryptAll unseals every file using kr's identity and reseals it
// for kr's full recipient list. Used by `rela keys add/remove`.
func reencryptAll(root string, kr *encryption.Keyring) error {
	recipients := kr.Recipients()
	return walkDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !encryption.LooksSealed(raw) {
			return fmt.Errorf("expected sealed file, got cleartext: %s", path)
		}
		cleartext, err := encryption.Unseal(raw, kr.Identity())
		if err != nil {
			return fmt.Errorf("unseal %s: %w", path, err)
		}
		sealed, err := encryption.Seal(cleartext, recipients)
		if err != nil {
			return err
		}
		return os.WriteFile(path, sealed, 0o644)
	})
}

// walkDataFiles invokes fn for every regular file under root's
// entities/, relations/, attachments/, and the fsstore index file.
// Hidden files and temp/backup files are skipped.
func walkDataFiles(root string, fn func(string) error) error {
	dirs := []string{
		filepath.Join(root, "entities"),
		filepath.Join(root, "relations"),
		filepath.Join(root, "attachments"),
	}
	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".new") || strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, "~") {
				return nil
			}
			return fn(path)
		}); err != nil {
			return err
		}
	}
	idx := filepath.Join(root, ".rela", "fsstore-index.json")
	if _, err := os.Stat(idx); err == nil {
		if err := fn(idx); err != nil {
			return err
		}
	}
	return nil
}

// writeEncryptionConfig writes the recipient list marker file under
// cacheDir. Content is a minimal list; the tool treats the file's
// presence as the "encryption is on" bit.
func writeEncryptionConfig(cacheDir string, recipients []string) error {
	var buf strings.Builder
	buf.WriteString("# This file's presence enables at-rest encryption for this rela repo.\n")
	buf.WriteString("# Recipient public keys live in <repo>/keys/<name>.pub.\n")
	buf.WriteString("recipients:\n")
	for _, r := range recipients {
		buf.WriteString("  - " + r + "\n")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(
		filepath.Join(cacheDir, app.EncryptionConfigFile),
		[]byte(buf.String()),
		0o644,
	)
}
