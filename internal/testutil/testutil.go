package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TempDirWithCleanup creates a temporary directory for testing.
// Uses t.TempDir() which automatically cleans up after the test.
func TempDirWithCleanup(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// CreateFile creates a file with the given content at the specified path.
// Creates parent directories as needed.
func CreateFile(t *testing.T, path, content string) {
	t.Helper()

	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// CreateDir creates a directory at the specified path.
// Creates parent directories as needed.
func CreateDir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}
}

// AssertFileExists checks that a file exists at the given path.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// AssertFileNotExists checks that a file does not exist at the given path.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to not exist: %s", path)
	}
}

// AssertNoError checks that an error is nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError checks that an error is not nil.
func AssertError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Error("expected error, got nil")
	}
}

// AssertEqual checks that two values are equal.
func AssertEqual(t *testing.T, got, want interface{}) {
	t.Helper()

	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// AssertNotEqual checks that two values are not equal.
func AssertNotEqual(t *testing.T, got, notWant interface{}) {
	t.Helper()

	if got == notWant {
		t.Errorf("got %v, expected different value", got)
	}
}

// AssertStringContains checks that a string contains a substring.
func AssertStringContains(t *testing.T, s, substr string) {
	t.Helper()

	if s == "" && substr != "" {
		t.Errorf("string is empty, expected to contain %q", substr)
		return
	}

	// Simple substring check
	found := false
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("string %q does not contain %q", s, substr)
	}
}

// AssertStringNotContains checks that a string does not contain a substring.
func AssertStringNotContains(t *testing.T, s, substr string) {
	t.Helper()

	// Simple substring check
	found := false
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			found = true
			break
		}
	}

	if found {
		t.Errorf("string %q unexpectedly contains %q", s, substr)
	}
}

// AssertLengthEqual checks that a slice or map has the expected length.
func AssertLengthEqual(t *testing.T, slice interface{}, expectedLen int) {
	t.Helper()

	var actualLen int
	switch v := slice.(type) {
	case []string:
		actualLen = len(v)
	case []interface{}:
		actualLen = len(v)
	case []error:
		actualLen = len(v)
	case map[string]bool:
		actualLen = len(v)
	case map[string]interface{}:
		actualLen = len(v)
	default:
		t.Fatalf("unsupported type for length check: %T", slice)
	}

	if actualLen != expectedLen {
		t.Errorf("length = %d, want %d", actualLen, expectedLen)
	}
}

// ReadFile reads a file and returns its contents as a string.
func ReadFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}

	return string(content)
}

// ChangeDir changes the current directory and returns a cleanup function
// that restores the original directory.
func ChangeDir(t *testing.T, dir string) func() {
	t.Helper()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Resolve symlinks (important on macOS where /tmp -> /private/tmp)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", dir, err)
	}

	if err := os.Chdir(resolvedDir); err != nil {
		t.Fatalf("failed to change directory to %s: %v", resolvedDir, err)
	}

	return func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Fatalf("failed to restore directory to %s: %v", originalWd, err)
		}
	}
}

// AssertIsDir checks that a path is a directory.
func AssertIsDir(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("failed to stat %s: %v", path, err)
		return
	}

	if !info.IsDir() {
		t.Errorf("%s is not a directory", path)
	}
}
