package storage

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemFS implements FS using an in-memory filesystem.
// It is safe for concurrent use.
// Primarily intended for testing.
type MemFS struct {
	mu    sync.RWMutex
	files map[string]*memFile  // path → file contents
	dirs  map[string]time.Time // directory path → mtime
	cwd   string               // current working directory for Getwd
}

type memFile struct {
	data    []byte
	perm    os.FileMode
	modTime time.Time
}

// NewMemFS returns a new in-memory filesystem rooted at "/".
func NewMemFS() *MemFS {
	return &MemFS{
		files: make(map[string]*memFile),
		dirs:  map[string]time.Time{"/": time.Now()},
		cwd:   "/",
	}
}

// SetCwd sets the working directory returned by Getwd.
func (m *MemFS) SetCwd(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cwd = dir
}

func (m *MemFS) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = cleanPath(path)
	f, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	// Return a copy to prevent mutation.
	data := make([]byte, len(f.data))
	copy(data, f.data)
	return data, nil
}

func (m *MemFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = cleanPath(path)

	// Check that parent directory exists.
	dir := filepath.Dir(path)
	if _, ok := m.dirs[dir]; !ok {
		return &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}

	// Copy data to prevent caller mutation.
	stored := make([]byte, len(data))
	copy(stored, data)

	now := time.Now()
	_, existed := m.files[path]
	m.files[path] = &memFile{
		data:    stored,
		perm:    perm,
		modTime: now,
	}
	if !existed {
		m.touchDir(dir, now)
	}
	return nil
}

func (m *MemFS) Remove(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = cleanPath(path)
	now := time.Now()

	// Check if it's a file.
	if _, ok := m.files[path]; ok {
		delete(m.files, path)
		m.touchDir(filepath.Dir(path), now)
		return nil
	}

	// Check if it's a directory.
	if _, ok := m.dirs[path]; ok {
		// Check if directory is empty.
		if m.hasChildren(path) {
			return &os.PathError{Op: "remove", Path: path, Err: os.ErrExist}
		}
		delete(m.dirs, path)
		m.touchDir(filepath.Dir(path), now)
		return nil
	}

	return &os.PathError{Op: "remove", Path: path, Err: os.ErrNotExist}
}

func (m *MemFS) Rename(oldpath, newpath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldpath = cleanPath(oldpath)
	newpath = cleanPath(newpath)

	// Check that new parent exists.
	newDir := filepath.Dir(newpath)
	if _, ok := m.dirs[newDir]; !ok {
		return &os.PathError{Op: "rename", Path: newpath, Err: os.ErrNotExist}
	}

	now := time.Now()

	// Handle file rename.
	if f, ok := m.files[oldpath]; ok {
		m.files[newpath] = f
		delete(m.files, oldpath)
		m.touchDir(filepath.Dir(oldpath), now)
		if filepath.Dir(oldpath) != newDir {
			m.touchDir(newDir, now)
		}
		return nil
	}

	// Handle directory rename: move all children.
	if _, ok := m.dirs[oldpath]; ok {
		// Collect all paths under oldpath.
		var filesToMove []string
		var dirsToMove []string

		for p := range m.files {
			if p == oldpath || strings.HasPrefix(p, oldpath+"/") {
				filesToMove = append(filesToMove, p)
			}
		}
		for p := range m.dirs {
			if p == oldpath || strings.HasPrefix(p, oldpath+"/") {
				dirsToMove = append(dirsToMove, p)
			}
		}

		// Move files.
		for _, p := range filesToMove {
			suffix := strings.TrimPrefix(p, oldpath)
			m.files[newpath+suffix] = m.files[p]
			delete(m.files, p)
		}

		// Move directories.
		for _, p := range dirsToMove {
			suffix := strings.TrimPrefix(p, oldpath)
			m.dirs[newpath+suffix] = m.dirs[p]
			delete(m.dirs, p)
		}

		m.touchDir(filepath.Dir(oldpath), now)
		if filepath.Dir(oldpath) != newDir {
			m.touchDir(newDir, now)
		}

		return nil
	}

	return &os.PathError{Op: "rename", Path: oldpath, Err: os.ErrNotExist}
}

func (m *MemFS) Stat(path string) (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = cleanPath(path)

	if f, ok := m.files[path]; ok {
		return &memFileInfo{
			name:    filepath.Base(path),
			size:    int64(len(f.data)),
			mode:    f.perm,
			modTime: f.modTime,
			isDir:   false,
		}, nil
	}

	if mtime, ok := m.dirs[path]; ok {
		return &memFileInfo{
			name:    filepath.Base(path),
			size:    0,
			mode:    os.ModeDir | 0755,
			modTime: mtime,
			isDir:   true,
		}, nil
	}

	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func (m *MemFS) MkdirAll(path string, _ os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = cleanPath(path)
	now := time.Now()

	// Create all path components.
	parts := strings.Split(path, "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			current = "/"
			continue
		}
		if current == "/" {
			current = "/" + part
		} else {
			current = current + "/" + part
		}
		if _, exists := m.dirs[current]; !exists {
			m.dirs[current] = now
			// Touch parent when creating a new subdir.
			m.touchDir(filepath.Dir(current), now)
		}
	}
	return nil
}

func (m *MemFS) ReadDir(path string) ([]os.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = cleanPath(path)

	if _, ok := m.dirs[path]; !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}

	prefix := path
	if prefix != "/" {
		prefix += "/"
	}

	seen := make(map[string]bool)
	entries := make([]os.DirEntry, 0)

	// Find direct child files.
	for p, f := range m.files {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := strings.TrimPrefix(p, prefix)
		if strings.Contains(rest, "/") {
			continue // Not a direct child.
		}
		if rest == "" {
			continue
		}
		if seen[rest] {
			continue
		}
		seen[rest] = true
		entries = append(entries, &memDirEntry{
			name:  rest,
			isDir: false,
			info: &memFileInfo{
				name:    rest,
				size:    int64(len(f.data)),
				mode:    f.perm,
				modTime: f.modTime,
				isDir:   false,
			},
		})
	}

	// Find direct child directories.
	for p, dirMtime := range m.dirs {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := strings.TrimPrefix(p, prefix)
		if rest == "" {
			continue
		}
		if strings.Contains(rest, "/") {
			continue // Not a direct child.
		}
		if seen[rest] {
			continue
		}
		seen[rest] = true
		entries = append(entries, &memDirEntry{
			name:  rest,
			isDir: true,
			info: &memFileInfo{
				name:    rest,
				mode:    os.ModeDir | 0755,
				modTime: dirMtime,
				isDir:   true,
			},
		})
	}

	// Sort by name (os.ReadDir contract).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	return entries, nil
}

func (m *MemFS) Walk(root string, fn filepath.WalkFunc) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	root = cleanPath(root)

	// Build sorted list of all paths under root.
	paths := m.collectPaths(root)
	sort.Strings(paths)

	i := 0
	for i < len(paths) {
		p := paths[i]
		info, _ := m.statLocked(p)
		if info == nil {
			i++
			continue
		}
		err := fn(p, info, nil)
		if err != nil {
			if errors.Is(err, filepath.SkipDir) && info.IsDir() {
				// Skip all entries under this directory.
				prefix := p + "/"
				i++
				for i < len(paths) && strings.HasPrefix(paths[i], prefix) {
					i++
				}
				continue
			}
			return err
		}
		i++
	}

	return nil
}

func (m *MemFS) Open(path string) (io.ReadCloser, error) {
	data, err := m.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (m *MemFS) Getwd() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cwd, nil
}

// hasChildren checks if a directory has any child files or subdirectories.
// Must be called with m.mu held.
func (m *MemFS) hasChildren(dir string) bool {
	prefix := dir
	if prefix != "/" {
		prefix += "/"
	}
	for p := range m.files {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	for p := range m.dirs {
		if p != dir && strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// touchDir updates a directory's mtime. Must be called with m.mu held.
func (m *MemFS) touchDir(dir string, t time.Time) {
	if _, ok := m.dirs[dir]; ok {
		m.dirs[dir] = t
	}
}

// collectPaths returns all file and directory paths at or under root, in no particular order.
// Must be called with m.mu held (at least RLock).
func (m *MemFS) collectPaths(root string) []string {
	var paths []string

	prefix := root
	if prefix != "/" {
		prefix += "/"
	}

	// Add root itself if it exists.
	if _, ok := m.dirs[root]; ok {
		paths = append(paths, root)
	} else if _, ok := m.files[root]; ok {
		// Root is a file, just return that.
		return []string{root}
	} else {
		return nil
	}

	// Add directories under root.
	for p := range m.dirs {
		if strings.HasPrefix(p, prefix) {
			paths = append(paths, p)
		}
	}

	// Add files under root.
	for p := range m.files {
		if strings.HasPrefix(p, prefix) {
			paths = append(paths, p)
		}
	}

	return paths
}

// statLocked returns FileInfo without acquiring the lock.
// Must be called with m.mu held.
func (m *MemFS) statLocked(path string) (os.FileInfo, error) {
	if f, ok := m.files[path]; ok {
		return &memFileInfo{
			name:    filepath.Base(path),
			size:    int64(len(f.data)),
			mode:    f.perm,
			modTime: f.modTime,
			isDir:   false,
		}, nil
	}
	if mtime, ok := m.dirs[path]; ok {
		return &memFileInfo{
			name:    filepath.Base(path),
			mode:    os.ModeDir | 0755,
			modTime: mtime,
			isDir:   true,
		}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

// cleanPath normalizes a path using filepath.Clean and ensures it's absolute-style.
func cleanPath(path string) string {
	return filepath.Clean(path)
}

// memFileInfo implements os.FileInfo for in-memory files.
type memFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *memFileInfo) Name() string       { return fi.name }
func (fi *memFileInfo) Size() int64        { return fi.size }
func (fi *memFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *memFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *memFileInfo) IsDir() bool        { return fi.isDir }
func (fi *memFileInfo) Sys() interface{}   { return nil }

// memDirEntry implements os.DirEntry for in-memory directories.
type memDirEntry struct {
	name  string
	isDir bool
	info  os.FileInfo
}

func (de *memDirEntry) Name() string               { return de.name }
func (de *memDirEntry) IsDir() bool                { return de.isDir }
func (de *memDirEntry) Type() fs.FileMode          { return de.info.Mode().Type() }
func (de *memDirEntry) Info() (os.FileInfo, error) { return de.info, nil }

// Compile-time check that MemFS implements FS.
var _ FS = (*MemFS)(nil)
