package encryption

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// ResumeInterruptedRotation is called at store-open time to recover
// from a crash that happened during `rela keys add` or `rela keys
// remove`. The normal, no-crash case is a one-line no-op — a
// missing sentinel means nothing is in flight.
//
// Recovery semantics:
//
//   - If no sentinel exists: return nil.
//   - If a sentinel exists whose ToVersion matches the loaded
//     keyring's version: the rotation finished but the sentinel
//     delete didn't happen (crash between WriteRecipientsFile and
//     DeleteResealSentinel, or a DeleteResealSentinel error the
//     initiator logged). Delete the sentinel and return.
//   - If a sentinel exists whose ToVersion exceeds the loaded
//     keyring's version: the rotation was interrupted before
//     recipients.age was rewritten. Re-run the walk (idempotent
//     for files already at the new version), rewrite
//     recipients.age from the sentinel's new-recipient list, and
//     delete the sentinel.
//   - Any other state (ToVersion < current, validation failure) is
//     a programming error or an adversary-planted sentinel;
//     return a loud error.
//
// After a successful return the caller can treat the keyring as
// authoritative again — but a recovery path that ran MUST reload
// the keyring because recipients.age was just rewritten. The
// caller is responsible for that reload.
func ResumeInterruptedRotation(repoRoot string, kr *Keyring, us userstate.FSService) (resumed bool, err error) {
	sentinel, err := ReadResealSentinel(us)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("resume rotation: read sentinel: %w", err)
	}

	// Sanity: the sentinel must describe THIS repo.
	if sentinel.RepoRoot != repoRoot {
		return false, fmt.Errorf(
			"resume rotation: sentinel repo_root %q != current repo %q (machine-local state drift?)",
			sentinel.RepoRoot, repoRoot)
	}

	// Completed-but-not-cleaned case: recipients.age is already at
	// to_version. Just sweep the sentinel.
	if sentinel.ToVersion == kr.Version() {
		return false, DeleteResealSentinel(us)
	}

	// Mid-flight case: we need to resume. kr carries the OLD
	// recipient set (what we're migrating FROM); sentinel carries
	// the NEW recipient set (what we're migrating TO).
	if sentinel.ToVersion < kr.Version() {
		return false, fmt.Errorf(
			"resume rotation: sentinel to_version %d is older than current version %d "+
				"(sentinel appears stale; delete %s manually if you're sure)",
			sentinel.ToVersion, kr.Version(), us.Path(resealSentinelKey))
	}
	if sentinel.FromVersion != kr.Version() {
		return false, fmt.Errorf(
			"resume rotation: sentinel from_version %d != current version %d",
			sentinel.FromVersion, kr.Version())
	}

	newRecipients, err := sentinel.NewRecipientList()
	if err != nil {
		return false, fmt.Errorf("resume rotation: parse new recipients: %w", err)
	}

	if err := resealAllForRotation(repoRoot, kr, newRecipients, sentinel.ToVersion); err != nil {
		return true, fmt.Errorf("resume rotation: %w", err)
	}

	newRF := &RecipientsFile{
		Version:    sentinel.ToVersion,
		RepoID:     kr.RepoID(),
		Recipients: sentinel.NewRecipients,
	}
	if err := WriteRecipientsFile(filepath.Join(repoRoot, RecipientsFileName), newRF); err != nil {
		return true, fmt.Errorf("resume rotation: rewrite %s: %w", RecipientsFileName, err)
	}
	if err := DeleteResealSentinel(us); err != nil {
		return true, fmt.Errorf("resume rotation: delete sentinel: %w", err)
	}
	return true, nil
}

// resealAllForRotation walks the data-file tree and re-seals every
// sealed file under newRecipients at newVersion. Idempotent: files
// already sealed at newVersion (the rotation completed them before
// the crash) are left alone. Uses the two-phase .rewrap.new staging
// pattern so a crash during recovery leaves a recognizable state
// for the NEXT recovery attempt.
func resealAllForRotation(root string, kr *Keyring, newRecipients []Recipient, newVersion int) error {
	var rewrapPaths []string
	walkErr := walkRotationDataFiles(root, func(path string) error {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !LooksSealed(raw) {
			return fmt.Errorf("expected sealed file, got cleartext: %s", path)
		}

		// Peek the version to skip files already migrated.
		body, skipReason, err := readAndClassify(root, path, raw, kr.Identity(), newVersion)
		if err != nil {
			return fmt.Errorf("classify %s: %w", path, err)
		}
		if skipReason != "" {
			return nil
		}

		sealed, err := sealWithHeader(root, path, body, newRecipients, newVersion)
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
	if walkErr != nil {
		for _, p := range rewrapPaths {
			_ = os.Remove(p + ".rewrap.new")
		}
		return walkErr
	}
	for _, p := range rewrapPaths {
		if err := os.Rename(p+".rewrap.new", p); err != nil {
			return fmt.Errorf("rename %s: %w", p, err)
		}
	}
	return nil
}

// readAndClassify unseals `raw` with `identity`, extracts the
// header, and returns (body, skipReason, err). skipReason is
// non-empty when the file is already at newVersion (so the caller
// should skip it) or when some recovery-specific condition
// applies. A decrypt failure that's a no-matching-key means the
// file was already rotated under the NEW key set — also a skip.
func readAndClassify(
	root, path string, raw []byte, identity Identity, newVersion int,
) (body []byte, skipReason string, err error) {
	plaintext, err := Unseal(raw, identity)
	if err != nil {
		if IsNoMatchingKey(err) {
			// Already sealed under the new recipient set; we
			// can't decrypt but we can infer it's been rotated.
			// Leave it alone.
			return nil, "already rotated to new recipients", nil
		}
		return nil, "", err
	}
	h, body, err := ParseHeader(plaintext)
	if err != nil {
		return nil, "", err
	}
	// Verify the path matches — otherwise we'd re-seal under the
	// wrong path and permanently lock in a swap. Fail loud.
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, "", err
	}
	expected := filepath.ToSlash(rel)
	if h.Path != expected {
		return nil, "", fmt.Errorf("%w: header path %q != file path %q",
			ErrFileRelocated, h.Path, expected)
	}
	if h.Version == newVersion {
		return nil, "already at new version", nil
	}
	return body, "", nil
}

// sealWithHeader is the rotation-local equivalent of
// cryptofs.FS.WriteFile's sealing path. Duplicated here (instead
// of calling cryptofs) so the encryption package stays free of
// cryptofs dependencies and recovery doesn't require building a
// throwaway cryptofs.FS just to seal bytes.
func sealWithHeader(root, absPath string, body []byte, recipients []Recipient, version int) ([]byte, error) {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return nil, fmt.Errorf("compute relative path for %s: %w", absPath, err)
	}
	h := &Header{Version: version, Path: filepath.ToSlash(rel)}
	enc := h.Encode()
	plaintext := make([]byte, 0, len(enc)+len(body))
	plaintext = append(plaintext, enc...)
	plaintext = append(plaintext, body...)
	return Seal(plaintext, recipients)
}

// walkRotationDataFiles walks root's entities/, relations/,
// attachments/, and .rela/fsstore-index.json — every file the
// rotation needs to re-seal. Skips temp/hidden files matching the
// same patterns as the CLI's walkDataFiles. Factored into the
// encryption package so recovery doesn't need to pull in any cli
// dependency.
func walkRotationDataFiles(root string, fn func(string) error) error {
	dirs := []string{
		filepath.Join(root, "entities"),
		filepath.Join(root, "relations"),
		filepath.Join(root, "attachments"),
	}
	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			if shouldSkipRotationFile(d.Name()) {
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

func shouldSkipRotationFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	for _, suffix := range []string{".new", ".tmp", ".bak", ".rewrap.new", "~"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
