package storage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RootedFS is an FS bound to a validated root directory. Path-like
// arguments to its methods are interpreted as KEYS relative to the
// root. Every call runs keys through resolve(), which is the
// string-level path-validation barrier.
//
// RootedFS deliberately does NOT implement storage.FS. Functions that
// accept *RootedFS get a compile-time claim that keys have been
// validated; functions that accept storage.FS see raw, caller-validated
// paths. Raw FS usage is still available for internal storage
// decorators (SafeFS, MemFS) and for the wiring layer that constructs
// RootedFS.
//
// Internally the same claim is carried by [ValidatedPath]: resolve
// returns one, and RootedFS reaches the wrapped FS only through
// validatedFS, whose methods take a ValidatedPath. So there is no
// path — not even an accidental one inside this type — from an
// unvalidated key to an os call.
//
// Getwd is intentionally omitted — it does not fit the keyed-access
// model.
//
// # Security
//
// resolve() is a STRING-level validator: it rejects traversal in the
// key (".."/absolute/backslash/control chars/drive letters/Windows
// reserved names) before joining with the root. It does NOT resolve
// symlinks. A symlink inside the root pointing outside the root is
// still followed by the underlying OS; this is out of scope for this
// type. The threat model is "caller-supplied key contains traversal
// syntax", not "attacker has write access to the root directory".
//
// # Concurrency
//
// RootedFS is stateless after construction (root and fs are immutable)
// and inherits the concurrency semantics of the underlying FS.
type RootedFS struct {
	vfs  validatedFS
	fs   FS // retained only for SupportsStreaming's backend sniff
	root string
}

// NewRootedFS returns a RootedFS bound to root. root is cleaned and
// made absolute; empty root or a nil fs is rejected.
func NewRootedFS(fs FS, root string) (*RootedFS, error) {
	if fs == nil {
		return nil, errors.New("storage: RootedFS fs must not be nil")
	}
	if root == "" {
		return nil, errors.New("storage: RootedFS root must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve RootedFS root: %w", err)
	}
	return &RootedFS{vfs: validatedFS{fs: fs}, fs: fs, root: filepath.Clean(abs)}, nil
}

// windowsReserved lists Windows reserved device names (case-insensitive,
// matched against the stem before the first '.'). Rejecting these in
// keys prevents portability surprises: a key that works on POSIX would
// otherwise open a device handle on Windows.
var windowsReserved = map[string]bool{
	"con": true, "nul": true, "prn": true, "aux": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// resolve validates key and returns its absolute path. Rules:
//   - reject empty
//   - reject control characters (< 0x20 or 0x7f)
//   - reject backslash (forward slash only)
//   - reject colon (blocks drive letters and Windows ADS syntax)
//   - reject absolute paths (leading '/')
//   - reject empty, ".", or ".." segments
//   - reject Windows reserved device names (CON, NUL, COM1–9, etc.)
//
// Valid keys produce filepath.Join(root, key).
func (r *RootedFS) resolve(key string) (ValidatedPath, error) {
	if key == "" {
		return ValidatedPath{}, errors.New("storage: key must not be empty")
	}
	for _, c := range key {
		if c < 0x20 || c == 0x7f {
			return ValidatedPath{}, errors.New("storage: control character (including NUL) not allowed in key")
		}
	}
	if strings.ContainsRune(key, '\\') {
		return ValidatedPath{}, errors.New("storage: backslash not allowed in key (use forward slash)")
	}
	if strings.ContainsRune(key, ':') {
		return ValidatedPath{}, errors.New("storage: colon not allowed in key (blocks Windows drive letters and ADS)")
	}
	if strings.HasPrefix(key, "/") {
		return ValidatedPath{}, errors.New("storage: key must be relative")
	}
	for seg := range strings.SplitSeq(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return ValidatedPath{}, errors.New("storage: traversal or empty segment not allowed in key")
		}
		stem := strings.ToLower(seg)
		if i := strings.Index(stem, "."); i >= 0 {
			stem = stem[:i]
		}
		if windowsReserved[stem] {
			return ValidatedPath{}, fmt.Errorf("storage: Windows reserved name %q not allowed in key", seg)
		}
	}
	return r.contain(filepath.Join(r.root, key))
}

// contain is the final barrier: it cleans the joined path and verifies the
// result is still the root or strictly beneath it, and it is the ONLY place
// (besides rootPath, which uses the root itself) that mints a ValidatedPath.
//
// The segment checks above already reject every traversal form, so this
// should be unreachable — that is the point. It converts "the rules above are
// exhaustive" from a claim into a checked postcondition, so a future edit that
// loosens one of them fails closed here instead of escaping the root. It is
// also what makes the containment legible to static analysis, which cannot
// infer it from the string-level rules.
func (r *RootedFS) contain(joined string) (ValidatedPath, error) {
	clean := filepath.Clean(joined)
	// Compare against the root with exactly one trailing separator. Building
	// the prefix as root+sep breaks when the root IS the separator ("/"), the
	// case a container or a test tmpdir at the filesystem root actually hits:
	// "//entities" matches nothing and every path looks like an escape.
	prefix := strings.TrimSuffix(r.root, string(filepath.Separator)) + string(filepath.Separator)
	if clean != r.root && !strings.HasPrefix(clean, prefix) {
		return ValidatedPath{}, fmt.Errorf("storage: resolved path %q escapes root %q", clean, r.root)
	}
	return ValidatedPath{p: clean}, nil
}

// rootPath is the root itself as a ValidatedPath. NewRootedFS made it
// absolute and cleaned it, so it satisfies the invariant by
// construction — it is not derived from a caller-supplied key at all.
func (r *RootedFS) rootPath() ValidatedPath {
	return ValidatedPath{p: r.root}
}

// parent returns the directory containing v, which is still inside the root
// (v is contained, and a contained path's parent is the root or below it).
// It goes through contain anyway rather than minting a ValidatedPath
// directly: keeping construction to a single checked chokepoint is what makes
// the invariant hold by inspection.
//
// A parent that somehow escaped would be a bug in contain, so this returns
// the root rather than a path outside it — MkdirAll on the root is a harmless
// no-op, whereas propagating an out-of-root directory is not.
func (r *RootedFS) parent(v ValidatedPath) ValidatedPath {
	p, err := r.contain(filepath.Dir(v.p))
	if err != nil {
		return r.rootPath()
	}
	return p
}

// ReadFile reads the file at key.
func (r *RootedFS) ReadFile(key string) ([]byte, error) {
	full, err := r.resolve(key)
	if err != nil {
		return nil, err
	}
	return r.vfs.ReadFile(full)
}

// WriteFile writes data to key, creating parent directories if needed.
// Matches SafeFS.WriteFile semantics: parent dirs are auto-created so
// callers never need to MkdirAll before a write.
func (r *RootedFS) WriteFile(key string, data []byte, perm os.FileMode) error {
	full, err := r.resolve(key)
	if err != nil {
		return err
	}
	if err := r.vfs.MkdirAll(r.parent(full), 0o755); err != nil {
		return err
	}
	return r.vfs.WriteFile(full, data, perm)
}

// OpenForWrite opens key for streaming writes, creating parent
// directories and truncating the file if it exists. The returned
// WriteCloser wraps the underlying os.File; the caller is responsible
// for Close and, if atomic-rename semantics are desired, for writing
// to a temp key and renaming on successful Close.
//
// Unlike WriteFile, OpenForWrite does NOT go through SafeFS's
// atomic-rename-and-fsync path. It's the streaming counterpart, used
// when the data is too large to buffer in memory.
func (r *RootedFS) OpenForWrite(key string, perm os.FileMode) (io.WriteCloser, error) {
	full, err := r.resolve(key)
	if err != nil {
		return nil, err
	}
	if err := r.vfs.MkdirAll(r.parent(full), 0o755); err != nil {
		return nil, err
	}
	return r.vfs.OpenForWrite(full, perm)
}

// AbsPath resolves key to its absolute filesystem path. Used by
// integration points that must hand an absolute path to code outside
// the rooted abstraction — notably the fsnotify watcher, which takes
// an absolute directory to watch.
//
// Returns the resolve() error on invalid keys. Callers should treat
// the returned path as write-protected — do NOT pass it back into
// raw FS methods to bypass validation. If you need that, you're
// probably looking for OpenForWrite.
func (r *RootedFS) AbsPath(key string) (string, error) {
	full, err := r.resolve(key)
	if err != nil {
		return "", err
	}
	return full.String(), nil
}

// SupportsStreaming reports whether OpenForWrite can be used against
// the underlying filesystem. True for OsFS-backed stacks (direct or
// via SafeFS). False for MemFS-backed stacks (no on-disk file for
// os.OpenFile to open). Callers use this to choose between
// OpenForWrite (streaming) and WriteFile (buffered).
func (r *RootedFS) SupportsStreaming() bool {
	fs := r.fs
	for {
		switch v := fs.(type) {
		case *OsFS:
			return true
		case *SafeFS:
			fs = v.FS
		default:
			return false
		}
	}
}

// Remove removes the file or empty directory at key.
func (r *RootedFS) Remove(key string) error {
	full, err := r.resolve(key)
	if err != nil {
		return err
	}
	return r.vfs.Remove(full)
}

// Rename renames oldKey to newKey. Both keys are validated.
func (r *RootedFS) Rename(oldKey, newKey string) error {
	oldFull, err := r.resolve(oldKey)
	if err != nil {
		return err
	}
	newFull, err := r.resolve(newKey)
	if err != nil {
		return err
	}
	return r.vfs.Rename(oldFull, newFull)
}

// Stat returns file info for key.
func (r *RootedFS) Stat(key string) (os.FileInfo, error) {
	full, err := r.resolve(key)
	if err != nil {
		return nil, err
	}
	return r.vfs.Stat(full)
}

// MkdirAll creates the directory at key and any missing parents.
func (r *RootedFS) MkdirAll(key string, perm os.FileMode) error {
	full, err := r.resolve(key)
	if err != nil {
		return err
	}
	return r.vfs.MkdirAll(full, perm)
}

// ReadDir reads the directory entries at key.
func (r *RootedFS) ReadDir(key string) ([]os.DirEntry, error) {
	full, err := r.resolve(key)
	if err != nil {
		return nil, err
	}
	return r.vfs.ReadDir(full)
}

// Open opens the file at key for reading.
func (r *RootedFS) Open(key string) (io.ReadCloser, error) {
	full, err := r.resolve(key)
	if err != nil {
		return nil, err
	}
	return r.vfs.Open(full)
}

// Walk walks the subtree rooted at key. The callback receives keys
// (root-relative forward-slash paths), never absolute paths.
func (r *RootedFS) Walk(key string, fn fs.WalkDirFunc) error {
	full, err := r.resolve(key)
	if err != nil {
		return err
	}
	return r.vfs.Walk(full, r.relativize(fn))
}

// WalkAll walks the entire rooted tree.
//
// NOTE: the callback receives the root entry as ".", and every other
// entry as a root-relative forward-slash key. "." is NOT a valid key
// for any other RootedFS method — callers that want to read the root
// entry do so via ReadDir("<first subkey>") or similar, not by feeding
// "." back in.
func (r *RootedFS) WalkAll(fn fs.WalkDirFunc) error {
	return r.vfs.Walk(r.rootPath(), r.relativize(fn))
}

// relativize wraps a WalkDirFunc so it receives root-relative
// forward-slash keys instead of absolute paths. If filepath.Rel fails
// (e.g. cross-volume symlinks on Windows), the error is propagated
// rather than silently leaking an absolute path to the callback.
func (r *RootedFS) relativize(fn fs.WalkDirFunc) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, walkErr error) error {
		rel, err := filepath.Rel(r.root, path)
		if err != nil {
			return fmt.Errorf("rooted: callback path %q not under root %q: %w", path, r.root, err)
		}
		return fn(filepath.ToSlash(rel), d, walkErr)
	}
}
