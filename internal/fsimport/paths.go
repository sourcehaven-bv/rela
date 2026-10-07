package fsimport

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// resolvedPaths are the absolute, symlink-free source and target paths.
type resolvedPaths struct {
	source string
	target string
}

// resolvePaths checks the paths before anything is written.
//
// Containment is tested with os.SameFile on the real directories, walking
// ancestors, rather than by comparing strings: on a case-insensitive
// filesystem "/Work/proj" and "/work/PROJ" are the same directory, and a
// string comparison would let a target land inside its own source.
func resolvePaths(source, target string, owned []string) (resolvedPaths, error) {
	if source == "" || target == "" {
		return resolvedPaths{}, errors.New("both a source and a target directory are required")
	}
	src, err := filepath.Abs(source)
	if err != nil {
		return resolvedPaths{}, err
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return resolvedPaths{}, fmt.Errorf("source: %w", err)
	}
	info, err := os.Stat(src)
	if err != nil {
		return resolvedPaths{}, fmt.Errorf("source: %w", err)
	}
	if !info.IsDir() {
		return resolvedPaths{}, fmt.Errorf("source %s is not a directory", src)
	}
	if _, _, found := project.SchemaFileAt(src, storage.NewOsFS()); !found {
		return resolvedPaths{}, fmt.Errorf("source %s is not a rela project: no schema file", src)
	}
	for _, rel := range owned {
		if _, err = os.Lstat(filepath.Join(src, rel)); err == nil {
			return resolvedPaths{}, fmt.Errorf("source %s already has %s; it is not a filesystem project", src, rel)
		}
	}

	dst, err := filepath.Abs(target)
	if err != nil {
		return resolvedPaths{}, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(dst))
	if err != nil {
		return resolvedPaths{}, fmt.Errorf("target parent directory: %w", err)
	}
	name := filepath.Base(dst)
	if name == "." || name == string(filepath.Separator) || name == ".." {
		return resolvedPaths{}, fmt.Errorf("target %q does not name a directory", target)
	}
	dst = filepath.Join(parent, name)
	if _, err = os.Lstat(dst); err == nil {
		return resolvedPaths{}, fmt.Errorf("target %s already exists; choose a new directory", dst)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return resolvedPaths{}, fmt.Errorf("target: %w", err)
	}

	// The target does not exist yet, so test its parent: the target is
	// inside the source exactly when the parent is the source or below it.
	inside, err := isWithin(parent, src)
	if err != nil {
		return resolvedPaths{}, err
	}
	if inside {
		return resolvedPaths{}, fmt.Errorf("target %s is inside the source %s", dst, src)
	}
	// The source cannot be inside a target that does not exist yet. A target
	// naming the source itself, in any spelling the filesystem accepts,
	// already failed the existence check above.
	return resolvedPaths{source: src, target: dst}, nil
}

// isWithin reports whether dir is ancestor or lies below it.
func isWithin(dir, ancestor string) (bool, error) {
	anc, err := os.Stat(ancestor)
	if err != nil {
		return false, err
	}
	for cur := dir; ; {
		info, err := os.Stat(cur)
		if err != nil {
			return false, err
		}
		if os.SameFile(info, anc) {
			return true, nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			return false, nil
		}
		cur = next
	}
}

// makeStaging creates the staging directory beside the target, so the final
// rename stays on one filesystem.
func makeStaging(target string) (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(target),
		"."+filepath.Base(target)+".import-"+hex.EncodeToString(suffix[:]))
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".rela"), privateDir); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	return dir, nil
}
