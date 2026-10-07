package fsimport

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// Top-level entries of the source that are data, not project files. The
// store copy carries their content; fsimport never copies them as files.
var dataDirs = map[string]string{
	"entities":    "entity data, copied into the database",
	"relations":   "relation data, copied into the database",
	"attachments": "attachment data, copied into the database",
}

// notCopied lists source paths left out of the file copy, with the reason.
var notCopied = map[string]string{
	".git":                    "git history is not imported",
	"migrations/applied.json": "the applied-migration record moves into the database",
}

// Modes for what fsimport writes that only the owner may read: secrets,
// the audit log, and the .rela directory holding the database.
const (
	privateFile os.FileMode = 0o600
	privateDir  os.FileMode = 0o700
)

// relaConfigFiles are the operator-written files under .rela/ that stay files
// in the target. The mode caps the source's: secrets and the audit log are
// owner-only whatever the source had.
var relaConfigFiles = map[string]os.FileMode{
	"config.yaml":  0o644,
	"mail.yaml":    0o644,
	"ai.yaml":      0o644,
	"secrets.yaml": privateFile,
}

// relaAuditDir is copied whole, owner-only, like the audit backend writes it.
const relaAuditDir = "audit"

// copyProjectFiles copies the project's own files from src to dst: the schema,
// config, templates, scripts and everything else that is not data. Both roots
// are opened with os.OpenRoot, so no path in the source tree can make a read
// or write leave its root.
//
// Only regular files and directories are copied. A symlink or special file
// is listed rather than followed: following one could copy a file from
// outside the project into it.
func copyProjectFiles(src, dst string, rep *Report) error {
	srcRoot, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcRoot.Close() }()
	dstRoot, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstRoot.Close() }()

	err = fs.WalkDir(srcRoot.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if reason, ok := notCopied[p]; ok {
			if d.IsDir() {
				return fs.SkipDir
			}
			rep.skip(p, "%s", reason)
			return nil
		}
		if _, ok := dataDirs[p]; ok && d.IsDir() {
			return fs.SkipDir
		}
		if p == ".rela" {
			if !d.IsDir() {
				return errors.New("source .rela is not a directory")
			}
			return fs.SkipDir // copyRelaConfig handles it
		}
		if !d.IsDir() && usesGitCrypt(srcRoot, p) {
			rep.GitCrypt = true
		}
		return copyEntry(srcRoot, dstRoot, p, d, 0, rep)
	})
	if err != nil {
		return fmt.Errorf("copy project files: %w", err)
	}
	if err := copyRelaConfig(srcRoot, dstRoot, rep); err != nil {
		return err
	}
	return ensureGitignore(dstRoot)
}

// copyRelaConfig copies the allowlisted operator files from .rela/. Every
// other .rela entry is runtime state (copied into the database by copyState)
// or a cache; see classifyRela.
func copyRelaConfig(srcRoot, dstRoot *os.Root, rep *Report) error {
	for name, mode := range relaConfigFiles {
		p := ".rela/" + name
		info, err := srcRoot.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := copyEntry(srcRoot, dstRoot, p, fs.FileInfoToDirEntry(info), mode, rep); err != nil {
			return err
		}
	}
	auditPath := ".rela/" + relaAuditDir
	info, err := srcRoot.Lstat(auditPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		rep.skip(auditPath, "not a directory")
		return nil
	}
	return fs.WalkDir(srcRoot.FS(), auditPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return copyEntry(srcRoot, dstRoot, p, d, privateFile, rep)
	})
}

// copyEntry copies one walked entry with the source's permission bits,
// without setuid, setgid or sticky. mode, when non-zero, caps them: a file
// gets at most mode, and a directory at most 0700. Capping rather than
// replacing keeps a file the source holds owner-only that way.
func copyEntry(srcRoot, dstRoot *os.Root, p string, d fs.DirEntry, mode os.FileMode, rep *Report) error {
	switch {
	case d.Type()&fs.ModeSymlink != 0:
		rep.skip(p, "symbolic link; links are not followed")
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	case d.IsDir():
		info, err := d.Info()
		if err != nil {
			return err
		}
		perm := info.Mode().Perm()
		if mode != 0 {
			perm &= privateDir
		}
		if err := dstRoot.Mkdir(p, perm); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		return nil
	case !d.Type().IsRegular():
		rep.skip(p, "not a regular file")
		return nil
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	perm := info.Mode().Perm()
	if mode != 0 {
		perm &= mode
	}
	if err := copyFile(srcRoot, dstRoot, p, perm); err != nil {
		return err
	}
	rep.Files++
	return nil
}

func copyFile(srcRoot, dstRoot *os.Root, p string, perm os.FileMode) error {
	in, err := srcRoot.Open(p)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if dir := path.Dir(p); dir != "." {
		if mkErr := dstRoot.MkdirAll(dir, 0o755); mkErr != nil {
			return mkErr
		}
	}
	out, err := dstRoot.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	// The umask may have narrowed perm on create; set it exactly, so an
	// owner-only file is never wider and a normal file is not narrower.
	return dstRoot.Chmod(p, perm)
}

// ensureGitignore makes the target's .gitignore cover .rela/. The database,
// secrets and audit log all live there, and none of them belongs in git.
func ensureGitignore(dstRoot *os.Root) error {
	data, err := dstRoot.ReadFile(".gitignore")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		// Git ignores trailing spaces in a pattern but keeps leading ones,
		// so "  .rela/" ignores nothing and must not count.
		switch strings.TrimRight(line, " \r") {
		case ".rela", ".rela/", "/.rela", "/.rela/":
			return nil
		}
	}
	var add string
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		add = "\n"
	}
	add += ".rela/\n"
	f, err := dstRoot.OpenFile(".gitignore", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(add); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// usesGitCrypt reports whether a .gitattributes file routes files through
// git-crypt. The source's checkout holds them decrypted, so the target holds
// them in plain text: data in the database, and project files that a new
// repository without git-crypt set up would commit unencrypted.
func usesGitCrypt(root *os.Root, p string) bool {
	if path.Base(p) != ".gitattributes" {
		return false
	}
	data, err := root.ReadFile(p)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "filter=git-crypt")
}
