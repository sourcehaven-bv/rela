package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// keyFilePerm is the filesystem permission for private-key files
// (readable only by the owner).
const keyFilePerm os.FileMode = 0o600

// keysCmd is the top-level parent for all encryption-related
// commands.
//
// The authoritative recipient list for a rela project lives in
// <root>/recipients.age — an age-encrypted YAML blob sealed to
// itself. Its plaintext carries:
//
//   - version: monotonic counter, bumped on every recipient change
//   - repo_id: one-time UUID for keying per-machine state
//   - recipients: name → age public-key string
//
// Adding a recipient requires the caller to already be able to
// read recipients.age (i.e. already be a recipient). An adversary
// without a private key cannot silently add themselves: any attempt
// to replace recipients.age with a blob of their choosing makes it
// undecryptable for legitimate users, which surfaces loudly rather
// than silently expanding access.
//
// Public keys for proposed new recipients are passed to
// `rela keys add` via a file path (--pub-file); they never land
// inside the repo except as an entry in the encrypted recipients
// list. Private keys live outside the repo. See $RELA_KEY_FILE,
// .rela/key, and ~/.config/rela/key (resolution order).
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage at-rest encryption keys and recipients",
	Long: `Manage age-based at-rest encryption for this rela project.

A repo is encryption-enabled when <root>/recipients.age is present.
That file is the authoritative, encrypted list of recipients and
carries the repo's monotonic encryption version. Adding a
recipient requires decrypting the current file first, so only
existing recipients can expand the set.

Private keys live outside the repo (see $RELA_KEY_FILE,
.rela/key, ~/.config/rela/key in order).

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
		"name of the first recipient")
	keysInitCmd.Flags().StringVar(&keysInitPubFile, "pub-file", "",
		"path to a file containing the age public key for the first recipient")
	keysInitCmd.Flags().StringVar(&keysInitIdentity, "identity", "",
		"path to the private identity file to install at .rela/key")

	keysAddCmd.Flags().StringVar(&keysAddPubFile, "pub-file", "",
		"path to a file containing the age public key for the new recipient")
}

// readRecipientFromFile reads path and parses its contents as a
// single age recipient. Hybrid public keys are ~1959 characters, so
// they are distributed as files rather than command-line arguments.
func readRecipientFromFile(path string) (encryption.Recipient, string, error) {
	if path == "" {
		return nil, "", errors.New("--pub-file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", path, err)
	}
	s := strings.TrimSpace(string(data))
	rec, err := encryption.ParseRecipient(s)
	if err != nil {
		return nil, "", err
	}
	return rec, s, nil
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
		if err = os.WriteFile(pubPath, []byte(id.PublicRecipient().String()+"\n"), 0o644); err != nil {
			return err
		}
		// Serialize the private identity before writing anything. An
		// unsupported identity kind is a programming error, not a
		// runtime partial-failure we want to tolerate.
		priv, err := encryption.MarshalIdentity(id)
		if err != nil {
			return err
		}
		// Private key is sensitive; chmod 0600 and write atomically so
		// a crash mid-write cannot leave a truncated key on disk.
		if err = writeAtomic(keyPath, []byte(priv+"\n"), keyFilePerm); err != nil {
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

--recipient is a display name for the first recipient.
--pub-file is the path to the recipient's age public key file. Hybrid
  (post-quantum) public keys are ~2 KB so they are passed by path, not
  as a command-line string.
--identity, if set, copies the matching private key to .rela/key so
  this user becomes the default reader of the encrypted repo.

On success, <root>/recipients.age is written — an age-encrypted YAML
payload with the recipient list, the initial version (1), and a
one-time repo identifier. Every entity / relation / attachment file
is then sealed under the same recipient set.

The command refuses to proceed if the repo is already encrypted.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := validateRecipientName(keysInitRecipient); err != nil {
			return err
		}
		rec, pubStr, err := readRecipientFromFile(keysInitPubFile)
		if err != nil {
			return err
		}

		if err = ensureCleartextRepo(); err != nil {
			return err
		}

		// Resolve the repo identifier and open the user-state
		// service first — both the installed identity (below) and
		// every subsequent encryption-aware command need the user-
		// state directory in place. The resolver writes .rela/repo-id
		// when missing and cross-checks on re-init.
		repoID, err := resolveOrGenerateRepoID()
		if err != nil {
			return err
		}
		us, err := userstate.NewFSWithRepoID(projectCtx.Root, repoID)
		if err != nil {
			return err
		}

		// Install the private identity into the per-user, per-repo
		// directory under the OS user-config tree — outside the repo
		// and outside any directory-sync scope. The source file is
		// left untouched; we warn if it's inside the project tree
		// because that file is now a copy of the private key that
		// would still land on Dropbox / iCloud / OneDrive.
		if keysInitIdentity != "" {
			if err = copyFile(keysInitIdentity, us.Path("key"), keyFilePerm); err != nil {
				return err
			}
			if isInsideProject(keysInitIdentity, projectCtx.Root) {
				out.WriteMessage(
					"warning: --identity source %q is inside the project tree.",
					keysInitIdentity)
				out.WriteMessage(
					"A copy is now in the user-state directory (%s);",
					us.Path("key"))
				out.WriteMessage(
					"the source file is still in the project and is the cleartext")
				out.WriteMessage(
					"private key. Delete it if the project directory may be synced.")
			}
		}

		// Seal every data file under this recipient set first. If
		// the walk fails we haven't written recipients.age, so the
		// repo is still in a recognizably-cleartext state (the
		// partially sealed state is diagnosed by integrity.Verify on
		// next open).
		if err = sealAllFiles(projectCtx.Root, []encryption.Recipient{rec}); err != nil {
			return err
		}

		// Write the authoritative recipients file last — recipients.age
		// being present is the "encryption enabled" signal; do not set
		// it until the rest of the state is consistent.
		rf := &encryption.RecipientsFile{
			Version:    1,
			RepoID:     repoID,
			Recipients: map[string]string{keysInitRecipient: pubStr},
		}
		if err = encryption.WriteRecipientsFile(
			filepath.Join(projectCtx.Root, encryption.RecipientsFileName), rf); err != nil {
			return err
		}

		out.WriteSuccess("Encryption enabled. Repo is now sealed for %s.", keysInitRecipient)
		out.WriteMessage("")
		out.WriteMessage("The private identity is stored at:")
		out.WriteMessage("  %s", us.Path("key"))
		out.WriteMessage("")
		out.WriteMessage("This path is outside the project tree — safe to place the")
		out.WriteMessage("project directory on Dropbox/iCloud/OneDrive. Per-user caches")
		out.WriteMessage("(rendered documents, UI state) also live under the user-state")
		out.WriteMessage("directory. See docs/encryption.md for details.")
		return nil
	},
}

// resolveOrGenerateRepoID returns the canonical per-repo identifier
// for the current project, creating .rela/repo-id on first access.
// If the file is missing we generate a fresh id; if present we
// validate and reuse it. Used by keys-related commands at a point
// where the workspace has not yet constructed a user-state service
// (init time, pre-factory).
func resolveOrGenerateRepoID() (string, error) {
	id, err := project.ResolveRepoID(projectCtx.Root, "")
	if err != nil {
		return "", err
	}
	return id, nil
}

// openUserState returns a user-state service scoped to the current
// project. The service reads (and if missing, generates) the
// .rela/repo-id file. On encrypted repos the caller should call
// userstate.VerifyKeyringRepoID after the keyring loads to catch
// .rela/ directories copied in from other projects.
func openUserState() (userstate.FSService, error) {
	return userstate.Open(projectCtx.Root)
}

// isInsideProject reports whether path resolves to a location under
// projectRoot. Used for the --identity source warning: a key file
// inside the repo tree is the exact shape the ticket is moving
// away from.
func isInsideProject(path, projectRoot string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// --- decrypt ---

var keysDecryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Disable at-rest encryption for this project",
	Long:  `Unseal every entity, relation, and attachment, then remove <root>/recipients.age.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := ensureEncryptedRepo(); err != nil {
			return err
		}

		us, err := openUserState()
		if err != nil {
			return err
		}
		kr, err := encryption.LoadFromDir(projectCtx.Root, us)
		if err != nil {
			return err
		}
		if vErr := userstate.VerifyKeyringRepoID(projectCtx.Root, kr.RepoID()); vErr != nil {
			return vErr
		}
		if kr.LocalName() == "" {
			return errors.New("loaded identity is not in the recipient list; cannot unseal")
		}

		if err = unsealAllFiles(projectCtx.Root, kr); err != nil {
			return err
		}

		// Remove the authoritative file last so a crash before this
		// point still leaves a recognizable encrypted repo.
		if err = os.Remove(filepath.Join(projectCtx.Root, encryption.RecipientsFileName)); err != nil {
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
	Long: `Add a new recipient public key and re-encrypt every data file so the recipient can read.

The caller must be an existing recipient (have a working identity
for the current recipients.age). A new recipient cannot be added
by someone without a private key.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		if err := validateRecipientName(name); err != nil {
			return err
		}
		rec, pubStr, err := readRecipientFromFile(keysAddPubFile)
		if err != nil {
			return err
		}
		_ = rec // rec is validated by ParseRecipient; we only store the string form

		if err = ensureEncryptedRepo(); err != nil {
			return err
		}

		us, err := openUserState()
		if err != nil {
			return err
		}
		kr, err := encryption.LoadFromDir(projectCtx.Root, us)
		if err != nil {
			return err
		}
		if vErr := userstate.VerifyKeyringRepoID(projectCtx.Root, kr.RepoID()); vErr != nil {
			return vErr
		}
		if _, exists := kr.File().Recipients[name]; exists {
			return fmt.Errorf("recipient %s already exists", name)
		}

		// Mutate a copy of the current recipients file: bump version
		// and add the new recipient. The authoritative file is
		// written after the re-seal walk completes successfully.
		newRF := *kr.File()
		newRF.Recipients = make(map[string]string, len(kr.File().Recipients)+1)
		for k, v := range kr.File().Recipients {
			newRF.Recipients[k] = v
		}
		newRF.Recipients[name] = pubStr
		newRF.Version = kr.Version() + 1

		if err = rotateRecipients(projectCtx.Root, kr, us, &newRF, "keys add "+name); err != nil {
			return err
		}

		out.WriteSuccess("Added recipient %q and re-encrypted %d data files (version %d).",
			name, len(newRF.Recipients), newRF.Version)
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

		us, err := openUserState()
		if err != nil {
			return err
		}
		kr, err := encryption.LoadFromDir(projectCtx.Root, us)
		if err != nil {
			return err
		}
		if vErr := userstate.VerifyKeyringRepoID(projectCtx.Root, kr.RepoID()); vErr != nil {
			return vErr
		}
		if _, exists := kr.File().Recipients[name]; !exists {
			return fmt.Errorf("recipient %s not found", name)
		}
		if len(kr.File().Recipients) <= 1 {
			return errors.New("refusing to remove last recipient; run `rela keys decrypt` instead")
		}
		if name == kr.LocalName() {
			return errors.New("refusing to remove yourself (current identity); would lock you out")
		}

		newRF := *kr.File()
		newRF.Recipients = make(map[string]string, len(kr.File().Recipients)-1)
		for k, v := range kr.File().Recipients {
			if k == name {
				continue
			}
			newRF.Recipients[k] = v
		}
		newRF.Version = kr.Version() + 1

		if err = rotateRecipients(projectCtx.Root, kr, us, &newRF, "keys remove "+name); err != nil {
			return err
		}

		out.WriteSuccess("Removed recipient %q and re-encrypted %d data files (version %d).",
			name, len(newRF.Recipients), newRF.Version)
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
			out.WriteMessage("Repo is cleartext (no recipients.age).")
			return nil
		}
		us, err := openUserState()
		if err != nil {
			return err
		}
		kr, err := encryption.LoadFromDir(projectCtx.Root, us)
		if err != nil {
			return err
		}
		if vErr := userstate.VerifyKeyringRepoID(projectCtx.Root, kr.RepoID()); vErr != nil {
			return vErr
		}
		out.WriteMessage("Repo is encrypted (version %d, repo_id %s).", kr.Version(), kr.RepoID())
		out.WriteMessage("Recipients (%d):", len(kr.RecipientNames()))
		for _, n := range kr.RecipientNames() {
			marker := ""
			if n == kr.LocalName() {
				marker = " (you)"
			}
			r, _ := kr.Recipient(n)
			out.WriteMessage("  %s%s  %s", n, marker, r.String())
		}
		if kr.LocalName() == "" {
			out.WriteMessage("Local identity does not match any recipient.")
		}
		return nil
	},
}

// --- helpers ---

// validateRecipientName rejects names that would be inconvenient for
// CLI and YAML use. The old regime required these to be filesystem-
// safe for <name>.pub files; with recipients.age the constraint is
// weaker, but we keep a conservative character set to avoid
// surprises in CLI output, YAML encoding, and future extensions.
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
	return encryption.IsEnabled(projectCtx.Root)
}

func ensureCleartextRepo() error {
	enc, err := repoIsEncrypted()
	if err != nil {
		return err
	}
	if enc {
		return errors.New("repo is already encrypted (see <root>/recipients.age)")
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

// sealPlaintext wraps raw with a rela header (version + repo-
// relative path) and seals the combined bytes for recipients.
// Mirrors what cryptofs.FS.WriteFile does internally; we do it by
// hand here because the CLI seal walkers need to stage writes to
// <path>.rewrap.new while stamping the header with the FINAL path.
func sealPlaintext(root, absPath string, raw []byte, recipients []encryption.Recipient, version int) ([]byte, error) {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return nil, fmt.Errorf("compute relative path for %s: %w", absPath, err)
	}
	h := &encryption.Header{Version: version, Path: filepath.ToSlash(rel)}
	plaintext := append(h.Encode(), raw...)
	return encryption.Seal(plaintext, recipients)
}

// unsealPayload unseals sealed with identity, strips the rela
// header, and returns just the body bytes. Verifies the header's
// path matches absPath (ErrFileRelocated if not) but does NOT
// check rollback — CLI walkers operate on the whole tree at once
// and the rollback check belongs on the per-file store read path.
func unsealPayload(root, absPath string, sealed []byte, identity encryption.Identity) ([]byte, error) {
	plaintext, err := encryption.Unseal(sealed, identity)
	if err != nil {
		return nil, err
	}
	h, body, err := encryption.ParseHeader(plaintext)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", absPath, err)
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return nil, err
	}
	expected := filepath.ToSlash(rel)
	if h.Path != expected {
		return nil, fmt.Errorf("%w: header path %q != file path %q",
			encryption.ErrFileRelocated, h.Path, expected)
	}
	return body, nil
}

// sealAllFiles transitions a cleartext repo to sealed. Walks data
// dirs under root; seals every cleartext file for recipients at
// version 1 (initial encryption state). Any already-sealed file
// aborts the command — the invariant is "this repo is entirely
// cleartext before init", so a sealed file is a sign the repo is
// half-migrated from a prior interrupted run.
//
// Writes are atomic per file (temp + rename); a crash mid-walk
// leaves a partial state that integrity.Verify surfaces on next
// open. Recovery is manual (remove recipients.age if it was
// written, re-run `keys init`).
func sealAllFiles(root string, recipients []encryption.Recipient) error {
	return walkDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if encryption.LooksSealed(raw) {
			return fmt.Errorf("file %s is already sealed (repo may be half-migrated)", path)
		}
		sealed, err := sealPlaintext(root, path, raw, recipients, 1)
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
		body, err := unsealPayload(root, path, raw, kr.Identity())
		if err != nil {
			return fmt.Errorf("unseal %s: %w", path, err)
		}
		return writeAtomic(path, body, 0o644)
	})
}

// reencryptAll rewraps every sealed file under newRecipients at
// newVersion. Two-phase: first pass writes every path.rewrap.new
// sealed at the new version; second pass renames each .rewrap.new
// to its final path. A crash between phases leaves every
// .rewrap.new as an orphan sweepable on next open (fsstore's
// cleanupTempFiles handles ".new" suffixed files).
//
// Before the rename phase, every final file is still sealed at
// the OLD version / recipient set — readable by existing
// identities. After partial rename, every final file holds valid
// sealed bytes — either the new or old recipient set, never
// garbage. A subsequent re-run resumes from whichever phase the
// crash left off in.
func reencryptAll(root string, kr *encryption.Keyring, newRecipients []encryption.Recipient, newVersion int) error {
	// Phase 1: write .rewrap.new sealed at the new version for the
	// new recipient set. Header stamps the FINAL path (path), not
	// the staging path (path.rewrap.new), so phase-2 rename leaves
	// the header consistent with the file's location.
	var rewrapPaths []string
	err := walkDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !encryption.LooksSealed(raw) {
			return fmt.Errorf("expected sealed file, got cleartext: %s", path)
		}
		body, err := unsealPayload(root, path, raw, kr.Identity())
		if err != nil {
			return fmt.Errorf("unseal %s: %w", path, err)
		}
		sealed, err := sealPlaintext(root, path, body, newRecipients, newVersion)
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
		for _, p := range rewrapPaths {
			_ = os.Remove(p + ".rewrap.new")
		}
		return err
	}

	// Phase 2: rename each .rewrap.new -> path.
	for _, p := range rewrapPaths {
		if err := os.Rename(p+".rewrap.new", p); err != nil {
			return fmt.Errorf("rename %s: %w", p, err)
		}
	}
	return nil
}

// rotateRecipients orchestrates the full recipient-rotation flow
// used by `keys add` and `keys remove`:
//
//  1. Write an XDG-local sentinel describing the in-flight rotation
//     (from/to version, new recipient set, repo root, operation
//     label). Must land before any data-file mutation so a crash
//     mid-walk leaves a breadcrumb future rela invocations can
//     pick up and finish.
//  2. Run the two-phase reencryptAll walk (stage .rewrap.new files,
//     then rename each into place).
//  3. Write the new recipients.age — this is the commit point; a
//     crash before this has the walk visible but the official
//     recipient list still at the old version. Recovery re-runs
//     the walk (idempotent on already-rewritten files) and then
//     writes recipients.age.
//  4. Delete the sentinel. Any failure after step 3 leaves the
//     sentinel pointing at a completed rotation; the factory's
//     open-time recovery recognizes that case (sentinel.to_version
//     matches current recipients.age) and just deletes the
//     sentinel.
//
// operation is a human-readable label ("keys add alice", "keys
// remove bob") surfaced in diagnostics if recovery kicks in.
func rotateRecipients(root string, kr *encryption.Keyring, us userstate.FSService,
	newRF *encryption.RecipientsFile, operation string,
) error {
	newRecipients, err := newRF.RecipientList()
	if err != nil {
		return err
	}

	sentinel := &encryption.ResealSentinel{
		FromVersion:   kr.Version(),
		ToVersion:     newRF.Version,
		RepoRoot:      root,
		NewRecipients: newRF.Recipients,
		Operation:     operation,
	}
	if err := encryption.WriteResealSentinel(us, sentinel); err != nil {
		return fmt.Errorf("record rotation in progress: %w", err)
	}

	if err := reencryptAll(root, kr, newRecipients, newRF.Version); err != nil {
		// Keep the sentinel on walk failure so a rerun can pick
		// up from the partial state.
		return err
	}

	if err := encryption.WriteRecipientsFile(
		filepath.Join(root, encryption.RecipientsFileName), newRF); err != nil {
		return err
	}

	// Best-effort delete: the sentinel has served its purpose. A
	// failure here is survivable — the factory recognizes a
	// completed rotation by sentinel.to_version == keyring.version
	// and cleans up on next open.
	if err := encryption.DeleteResealSentinel(us); err != nil {
		out.WriteMessage("warning: %s left a stale sentinel; will be cleaned up on next rela invocation", operation)
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

// shouldSkipWalk reports whether a directory entry name should be
// skipped during walkDataFiles. Matches dotfiles and the temp /
// backup suffixes various editors and atomic-write implementations
// leave behind.
func shouldSkipWalk(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	for _, suffix := range []string{".new", ".tmp", ".bak", "~"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// walkDataFiles invokes fn for every regular file under root's
// entities/, relations/, attachments/, and the fsstore index file.
// Hidden files and temp/backup files are skipped. The authoritative
// recipients.age at the root is NOT walked — it has its own
// re-encrypt path (write a new one sealed to the new set).
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
			if shouldSkipWalk(d.Name()) {
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
