package storage_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// fsTestSuite runs the shared test suite against any FS implementation.
// setup returns the FS and a root directory path to use for tests.
func fsTestSuite(t *testing.T, name string, setup func(t *testing.T) (storage.FS, string)) { //nolint:thelper // this is a test suite runner, not a helper
	t.Run(name+"/ReadWriteFile", func(t *testing.T) {
		fs, root := setup(t)
		path := filepath.Join(root, "test.txt")
		data := []byte("hello world")

		if err := fs.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		got, err := fs.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})

	t.Run(name+"/ReadFileNotExists", func(t *testing.T) {
		fs, root := setup(t)
		_, err := fs.ReadFile(filepath.Join(root, "nonexistent.txt"))
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected IsNotExist error, got: %v", err)
		}
	})

	t.Run(name+"/WriteFileOverwrite", func(t *testing.T) {
		fs, root := setup(t)
		path := filepath.Join(root, "overwrite.txt")

		if err := fs.WriteFile(path, []byte("first"), 0644); err != nil {
			t.Fatalf("WriteFile first: %v", err)
		}
		if err := fs.WriteFile(path, []byte("second"), 0644); err != nil {
			t.Fatalf("WriteFile second: %v", err)
		}

		got, err := fs.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != "second" {
			t.Errorf("got %q, want %q", got, "second")
		}
	})

	t.Run(name+"/WriteFileParentNotExists", func(t *testing.T) {
		fs, root := setup(t)
		path := filepath.Join(root, "no", "such", "dir", "file.txt")
		err := fs.WriteFile(path, []byte("data"), 0644)
		if err == nil {
			t.Fatal("expected error when parent dir doesn't exist")
		}
	})

	t.Run(name+"/Remove", func(t *testing.T) {
		fs, root := setup(t)
		path := filepath.Join(root, "removeme.txt")

		if err := fs.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := fs.Remove(path); err != nil {
			t.Fatalf("Remove: %v", err)
		}

		_, err := fs.ReadFile(path)
		if !os.IsNotExist(err) {
			t.Errorf("expected IsNotExist after Remove, got: %v", err)
		}
	})

	t.Run(name+"/RemoveNotExists", func(t *testing.T) {
		fs, root := setup(t)
		err := fs.Remove(filepath.Join(root, "nonexistent.txt"))
		if err == nil {
			t.Fatal("expected error for Remove of nonexistent file")
		}
	})

	t.Run(name+"/Stat", func(t *testing.T) {
		fs, root := setup(t)
		path := filepath.Join(root, "statme.txt")
		data := []byte("hello")

		if err := fs.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		info, err := fs.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.IsDir() {
			t.Error("expected file, got directory")
		}
		if info.Size() != int64(len(data)) {
			t.Errorf("size = %d, want %d", info.Size(), len(data))
		}
		if info.Name() != "statme.txt" {
			t.Errorf("name = %q, want %q", info.Name(), "statme.txt")
		}
	})

	t.Run(name+"/StatNotExists", func(t *testing.T) {
		fs, root := setup(t)
		_, err := fs.Stat(filepath.Join(root, "nonexistent"))
		if !os.IsNotExist(err) {
			t.Errorf("expected IsNotExist, got: %v", err)
		}
	})

	t.Run(name+"/StatDir", func(t *testing.T) {
		fs, root := setup(t)
		subdir := filepath.Join(root, "subdir")
		if err := fs.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		info, err := fs.Stat(subdir)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run(name+"/MkdirAll", func(t *testing.T) {
		fs, root := setup(t)
		deep := filepath.Join(root, "a", "b", "c")

		if err := fs.MkdirAll(deep, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		// Should be able to write a file in the created directory.
		path := filepath.Join(deep, "file.txt")
		if err := fs.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile in deep dir: %v", err)
		}

		got, err := fs.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != "data" {
			t.Errorf("got %q, want %q", got, "data")
		}
	})

	t.Run(name+"/MkdirAllIdempotent", func(t *testing.T) {
		fs, root := setup(t)
		dir := filepath.Join(root, "idempotent")

		if err := fs.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll first: %v", err)
		}
		if err := fs.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll second: %v", err)
		}
	})

	t.Run(name+"/ReadDir", func(t *testing.T) {
		fs, root := setup(t)
		dir := filepath.Join(root, "readdir")
		if err := fs.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		// Create some files.
		for _, name := range []string{"b.txt", "a.txt", "c.txt"} {
			if err := fs.WriteFile(filepath.Join(dir, name), []byte(name), 0644); err != nil {
				t.Fatalf("WriteFile %s: %v", name, err)
			}
		}

		// Create a subdirectory.
		if err := fs.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
			t.Fatalf("MkdirAll subdir: %v", err)
		}

		entries, err := fs.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}

		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)

		expected := []string{"a.txt", "b.txt", "c.txt", "subdir"}
		if len(names) != len(expected) {
			t.Fatalf("got %d entries, want %d: %v", len(names), len(expected), names)
		}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("entry[%d] = %q, want %q", i, name, expected[i])
			}
		}

		// Check that subdir is reported as directory.
		for _, e := range entries {
			if e.Name() == "subdir" && !e.IsDir() {
				t.Error("expected subdir to be a directory")
			}
			if e.Name() == "a.txt" && e.IsDir() {
				t.Error("expected a.txt to be a file")
			}
		}
	})

	t.Run(name+"/ReadDirNotExists", func(t *testing.T) {
		fs, root := setup(t)
		_, err := fs.ReadDir(filepath.Join(root, "nonexistent"))
		if err == nil {
			t.Fatal("expected error for ReadDir of nonexistent dir")
		}
	})

	t.Run(name+"/Rename", func(t *testing.T) {
		fs, root := setup(t)
		oldPath := filepath.Join(root, "old.txt")
		newPath := filepath.Join(root, "new.txt")

		if err := fs.WriteFile(oldPath, []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := fs.Rename(oldPath, newPath); err != nil {
			t.Fatalf("Rename: %v", err)
		}

		// Old path should not exist.
		_, err := fs.Stat(oldPath)
		if !os.IsNotExist(err) {
			t.Errorf("old path should not exist after rename, got: %v", err)
		}

		// New path should have the data.
		got, err := fs.ReadFile(newPath)
		if err != nil {
			t.Fatalf("ReadFile new: %v", err)
		}
		if string(got) != "data" {
			t.Errorf("got %q, want %q", got, "data")
		}
	})

	t.Run(name+"/Walk", func(t *testing.T) {
		fs, root := setup(t)

		// Create a small tree.
		if err := fs.MkdirAll(filepath.Join(root, "walk", "sub"), 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := fs.WriteFile(filepath.Join(root, "walk", "a.txt"), []byte("a"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := fs.WriteFile(filepath.Join(root, "walk", "sub", "b.txt"), []byte("b"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		var walked []string
		walkRoot := filepath.Join(root, "walk")
		err := fs.Walk(walkRoot, func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			walked = append(walked, path)
			return nil
		})
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}

		// Should have walked: walk/, walk/a.txt, walk/sub/, walk/sub/b.txt
		if len(walked) != 4 {
			t.Fatalf("walked %d paths, want 4: %v", len(walked), walked)
		}

		// First entry should be the root.
		if walked[0] != walkRoot {
			t.Errorf("first walked path = %q, want %q", walked[0], walkRoot)
		}
	})

	t.Run(name+"/Open", func(t *testing.T) {
		fs, root := setup(t)
		path := filepath.Join(root, "openme.txt")
		data := []byte("contents")

		if err := fs.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		rc, err := fs.Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer rc.Close()

		buf := make([]byte, 100)
		n, _ := rc.Read(buf)
		if string(buf[:n]) != "contents" {
			t.Errorf("got %q, want %q", buf[:n], "contents")
		}
	})
}

func TestOsFS(t *testing.T) {
	fsTestSuite(t, "OsFS", func(t *testing.T) (storage.FS, string) {
		t.Helper()
		return storage.NewOsFS(), t.TempDir()
	})
}

func TestMemFS(t *testing.T) {
	fsTestSuite(t, "MemFS", func(t *testing.T) (storage.FS, string) {
		t.Helper()
		m := storage.NewMemFS()
		root := "/testroot"
		if err := m.MkdirAll(root, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		return m, root
	})
}
