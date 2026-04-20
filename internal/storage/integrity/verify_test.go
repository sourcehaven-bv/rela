package integrity_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/encryption"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/storage/integrity"
)

// Sealed is the first few bytes of an age v1 blob — enough for
// LooksSealed without generating a real identity in every test.
var sealedSentinel = []byte(encryption.SealedMagic)

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_AllSealed_WhenWantSealed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "entities")
	writeFile(t, filepath.Join(dir, "a.md"), sealedSentinel)
	writeFile(t, filepath.Join(dir, "b.md"), sealedSentinel)

	fs := storage.NewOsFS()
	if err := integrity.Verify(fs, true, []string{dir}); err != nil {
		t.Errorf("Verify should pass when every file is sealed: %v", err)
	}
}

func TestVerify_AllCleartext_WhenWantCleartext(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "entities")
	writeFile(t, filepath.Join(dir, "a.md"), []byte("---\nid: a\n---\n"))

	fs := storage.NewOsFS()
	if err := integrity.Verify(fs, false, []string{dir}); err != nil {
		t.Errorf("Verify should pass when every file is cleartext: %v", err)
	}
}

func TestVerify_CleartextFile_WhenWantSealed_ReturnsCleartextError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "entities")
	writeFile(t, filepath.Join(dir, "sealed.md"), sealedSentinel)
	writeFile(t, filepath.Join(dir, "stray.md"), []byte("---\nid: x\n---\n"))

	fs := storage.NewOsFS()
	err := integrity.Verify(fs, true, []string{dir})
	if err == nil {
		t.Fatal("expected error when a cleartext file exists under wantSealed=true")
	}
	if !errors.Is(err, integrity.ErrRepoHasCleartextFilesButEncryptionEnabled) {
		t.Errorf("wrong error class: %v", err)
	}
}

func TestVerify_SealedFile_WhenWantCleartext_ReturnsSealedError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "entities")
	writeFile(t, filepath.Join(dir, "cleartext.md"), []byte("---\nid: x\n---\n"))
	writeFile(t, filepath.Join(dir, "stray.md"), sealedSentinel)

	fs := storage.NewOsFS()
	err := integrity.Verify(fs, false, []string{dir})
	if err == nil {
		t.Fatal("expected error when a sealed file exists under wantSealed=false")
	}
	if !errors.Is(err, integrity.ErrRepoHasSealedFilesButNoConfig) {
		t.Errorf("wrong error class: %v", err)
	}
}

func TestVerify_MissingDirIsOK(t *testing.T) {
	root := t.TempDir()
	absent := filepath.Join(root, "never-created")

	fs := storage.NewOsFS()
	if err := integrity.Verify(fs, false, []string{absent}); err != nil {
		t.Errorf("missing dir should be fine: %v", err)
	}
	if err := integrity.Verify(fs, true, []string{absent}); err != nil {
		t.Errorf("missing dir should be fine even under wantSealed: %v", err)
	}
}

func TestVerify_EmptyDirListIsOK(t *testing.T) {
	fs := storage.NewOsFS()
	if err := integrity.Verify(fs, true, nil); err != nil {
		t.Errorf("empty dir list should be fine: %v", err)
	}
	if err := integrity.Verify(fs, false, []string{""}); err != nil {
		t.Errorf("empty-string dir should be skipped: %v", err)
	}
}

func TestVerify_SkipsTempAndDotfiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "entities")
	// Only the regular file matters. The others are temp/backup/dot
	// and must be ignored.
	writeFile(t, filepath.Join(dir, "real.md"), sealedSentinel)
	writeFile(t, filepath.Join(dir, "real.md.new"), []byte("cleartext temp"))
	writeFile(t, filepath.Join(dir, "real.md.tmp"), []byte("cleartext safefs temp"))
	writeFile(t, filepath.Join(dir, "real.md.bak"), []byte("cleartext backup"))
	writeFile(t, filepath.Join(dir, "editor~"), []byte("cleartext editor"))
	writeFile(t, filepath.Join(dir, ".dotfile"), []byte("cleartext dotfile"))

	fs := storage.NewOsFS()
	if err := integrity.Verify(fs, true, []string{dir}); err != nil {
		t.Errorf("skipped files should not trip the verifier: %v", err)
	}
}

func TestVerify_MultipleDirs(t *testing.T) {
	root := t.TempDir()
	entities := filepath.Join(root, "entities")
	relations := filepath.Join(root, "relations")
	writeFile(t, filepath.Join(entities, "a.md"), sealedSentinel)
	writeFile(t, filepath.Join(relations, "r.md"), []byte("cleartext")) // offender

	fs := storage.NewOsFS()
	err := integrity.Verify(fs, true, []string{entities, relations})
	if err == nil {
		t.Fatal("expected error when one dir has a cleartext file under wantSealed=true")
	}
	if !errors.Is(err, integrity.ErrRepoHasCleartextFilesButEncryptionEnabled) {
		t.Errorf("wrong error class: %v", err)
	}
}
