package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
	keysAddPubFile    string
	keysInitPubFile   string
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
	keysInitCmd.Flags().StringVar(&keysInitPubFile, "pub-file", "",
		"path to a file containing the age public key for the first recipient")
	keysInitCmd.Flags().StringVar(&keysInitIdentity, "identity", "",
		"path to the private identity file to install at .rela/key")

	keysAddCmd.Flags().StringVar(&keysAddPubFile, "pub-file", "",
		"path to a file containing the age public key for the new recipient")
}

// readRecipientFromFile reads path and parses its contents as a single
// age recipient. Hybrid public keys are ~1959 characters, so they are
// distributed as files rather than command-line arguments.
func readRecipientFromFile(path string) (encryption.Recipient, error) {
	if path == "" {
		return nil, errors.New("--pub-file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return encryption.ParseRecipient(strings.TrimSpace(string(data)))
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
		if err := os.WriteFile(keyPath, []byte(encryption.MarshalIdentity(id)+"\n"), keyFilePerm); err != nil {
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
  rela keys init --recipient <name> --pub-file <path> [--identity <path>]

--recipient must be a filename-stem (alphanumerics + hyphen/underscore).
--pub-file is the path to the recipient's age public key file. Hybrid
  (post-quantum) public keys are ~2 KB so they are passed by path, not
  as a command-line string.
--identity, if set, copies the private key to .rela/key so this user
  becomes the default reader of the encrypted repo.

The command refuses to proceed if the repo is already encrypted.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := validateRecipientName(keysInitRecipient); err != nil {
			return err
		}
		rec, err := readRecipientFromFile(keysInitPubFile)
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
			if err := ensureKeyGitignored(projectCtx.Root); err != nil {
				out.WriteMessage("warning: could not update .gitignore: %v", err)
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

		// Remove the encryption marker and the recipient pubkey files.
		// Only remove files we own (*.pub); leave any other contents
		// of keys/ alone (README, user-organized subdirs, etc.).
		if err := os.Remove(filepath.Join(projectCtx.CacheDir, encryption.ConfigFileName)); err != nil {
			return err
		}
		if err := removeRecipientPubFiles(filepath.Join(projectCtx.Root, "keys")); err != nil {
			return err
		}

		out.WriteSuccess("Encryption disabled. Repo is now cleartext.")
		return nil
	},
}

// removeRecipientPubFiles deletes every "*.pub" file under keysDir
// and removes the directory itself only if it becomes empty. Any
// non-pub files or subdirectories are left untouched — users sometimes
// keep README.md, .gitignore, or offline-signed keys in this dir.
func removeRecipientPubFiles(keysDir string) error {
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	remainingFiles := 0
	for _, e := range entries {
		if e.IsDir() {
			remainingFiles++
			continue
		}
		if strings.HasSuffix(e.Name(), ".pub") {
			if err := os.Remove(filepath.Join(keysDir, e.Name())); err != nil {
				return err
			}
			continue
		}
		remainingFiles++
	}
	if remainingFiles == 0 {
		// Directory is empty now; clean up.
		_ = os.Remove(keysDir)
	}
	return nil
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
		rec, err := readRecipientFromFile(keysAddPubFile)
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
	_, err := os.Stat(filepath.Join(projectCtx.CacheDir, encryption.ConfigFileName))
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
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("refusing to overwrite existing file: %s", dst)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return writeAtomic(dst, data, perm)
}

// sealAllFiles transitions a cleartext repo to sealed. Walks data
// dirs under root; seals every cleartext file for recipients. Any
// already-sealed file aborts the command (the invariant is "this
// repo is entirely cleartext before init", so a sealed file is a
// sign the repo is half-migrated from a prior interrupted run).
//
// Writes are atomic per file (temp + rename), so a crash mid-walk
// leaves an all-or-nothing-per-file state: some files fully sealed
// under recipients, the rest still cleartext. The repo is in a
// partial state; recovery is manual (delete .rela/encryption.yaml
// if present, re-run `keys init`).
func sealAllFiles(root string, recipients []encryption.Recipient) error {
	return walkDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if encryption.LooksSealed(raw) {
			return fmt.Errorf("file %s is already sealed (repo may be half-migrated)", path)
		}
		sealed, err := encryption.Seal(raw, recipients)
		if err != nil {
			return err
		}
		return writeAtomic(path, sealed, 0o644)
	})
}

// unsealAllFiles walks the same set and unseals every sealed file,
// using kr's local identity. Per-file atomic writes.
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
			return fmt.Errorf("unseal %s: %w", path, err)
		}
		return writeAtomic(path, cleartext, 0o644)
	})
}

// reencryptAll rewraps every sealed file under kr's full recipient
// list. Two-phase: first pass writes every path.rewrap.new sealed
// under the new recipients; second pass renames each .rewrap.new to
// its final path. A crash between phases leaves every .rewrap.new
// as an orphan sweepable on next open (fsstore's cleanupTempFiles
// already deletes ".new" suffixed files).
//
// This keeps the repo in a recoverable state even if the walk is
// interrupted: before the rename phase, every final file is still
// sealed for the pre-rewrap recipient set (no data loss); after
// partial rename, fsstore can still open the repo (the new-recipient
// files decrypt for the new identity; the old-recipient files
// decrypt for anyone who was a recipient both before and after).
func reencryptAll(root string, kr *encryption.Keyring) error {
	recipients := kr.Recipients()

	// Phase 1: write .rewrap.new sealed under the new recipient set.
	var rewrapPaths []string
	err := walkDataFiles(root, func(path string) error {
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
		rewrapPath := path + ".rewrap.new"
		if err := os.WriteFile(rewrapPath, sealed, 0o644); err != nil {
			return err
		}
		rewrapPaths = append(rewrapPaths, path)
		return nil
	})
	if err != nil {
		// Roll back phase-1 partial: delete every .rewrap.new we wrote.
		for _, p := range rewrapPaths {
			_ = os.Remove(p + ".rewrap.new")
		}
		return err
	}

	// Phase 2: rename each .rewrap.new -> path. These renames are
	// individually atomic on POSIX; as a batch they're not atomic,
	// but every path either holds the new sealed bytes or the old
	// sealed bytes, never garbage.
	for _, p := range rewrapPaths {
		if err := os.Rename(p+".rewrap.new", p); err != nil {
			return fmt.Errorf("rename %s: %w", p, err)
		}
	}
	return nil
}

// writeAtomic writes data to path via a temp file and rename. If
// the function returns without error, path either holds data (on
// success) or its previous contents (on failure).
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ensureKeyGitignored appends `.rela/key` to <root>/.gitignore if it
// is not already matched by an existing line. Called after writing
// the private identity so it cannot be accidentally committed even
// when the user's existing rules (e.g. `.rela/`) already cover it.
//
// Design notes:
//   - If .gitignore does not exist, we create it. Committing a new
//     .gitignore is a valid side effect of `rela keys init` since the
//     user just asked the tool to manage private keys.
//   - Matching is literal line comparison, not git's glob semantics.
//     Overly strict: someone with `.rela/*` in .gitignore still gets
//     a redundant `.rela/key` line. Acceptable: false positives are
//     cheap (one extra comment line) and the security win is real.
func ensureKeyGitignored(root string) error {
	const pattern = ".rela/key"
	gitignorePath := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(gitignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}

	addition := ""
	if !strings.Contains(string(existing), "# rela encryption") {
		addition = "\n# rela encryption — never commit private identities\n"
	}
	addition += pattern + "\n"

	return os.WriteFile(gitignorePath, append(existing, []byte(addition)...), 0o644)
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
		filepath.Join(cacheDir, encryption.ConfigFileName),
		[]byte(buf.String()),
		0o644,
	)
}
