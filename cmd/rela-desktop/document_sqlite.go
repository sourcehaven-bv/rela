//go:build sqlite

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// Rela documents.
//
// A document is a single <name>.rela file: the project database, carrying the
// config and the data. It opens from wherever the user keeps it, like any
// other document.
//
// Each document also gets a private workspace under the user's config
// directory. It is the project root, and it holds no config files, so nothing
// on disk can shadow the config inside the document. Secrets live in the
// keychain and settings in the document (see hostconfig.go); the workspace
// is read only for a secrets.yaml, mail.yaml or ai.yaml an older version left
// there. SQLite's own lock and WAL files stay beside the document. Opening the file from its own folder instead
// would let a stray schema.yaml next to it win.

// documentExt is the extension of a rela document.
const documentExt = ".rela"

// workspaceMode keeps a document's workspace, which may hold an older
// secrets.yaml,
// private to the user.
const workspaceMode = 0o700

// documentsRoot is where document workspaces live. A variable so tests can
// point it at a temporary directory.
var documentsRoot = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Rela Desktop", "documents"), nil
}

// isRelaDocument reports whether path is a rela document: a regular file
// named *.rela. A *.rela directory is a project bundle, handled as a folder.
func isRelaDocument(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), documentExt) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// documentName is the name a document shows: its file name without .rela.
func documentName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// documentContext returns the project context for the document at path,
// creating its workspace. The file itself need not exist yet, which is what
// lets New from Template create one.
func documentContext(path string) (*project.Context, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := documentsRoot()
	if err != nil {
		return nil, fmt.Errorf("locate document workspaces: %w", err)
	}
	// Keyed by the document's path, so reopening a document finds its
	// secrets and index again.
	sum := sha256.Sum256([]byte(abs))
	ws := filepath.Join(root, hex.EncodeToString(sum[:8]))
	if err := os.MkdirAll(filepath.Join(ws, project.CacheDir), workspaceMode); err != nil {
		return nil, fmt.Errorf("create document workspace: %w", err)
	}
	paths, atErr := project.At(ws, storage.NewSafeFS(storage.NewOsFS()))
	if atErr != nil {
		return nil, atErr
	}
	paths.DatabaseFile = abs
	return paths, nil
}

// newDocumentFromTemplate creates a document at dest from the project in
// templateDir: its config, and its markdown data when it has any. dest must
// not exist. A failed creation removes what it wrote.
func newDocumentFromTemplate(ctx context.Context, templateDir, dest string) (err error) {
	if _, statErr := os.Lstat(dest); statErr == nil {
		return fmt.Errorf("%s already exists; choose a new name", filepath.Base(dest))
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	paths, err := documentContext(dest)
	if err != nil {
		return err
	}
	fsys := storage.NewSafeFS(storage.NewOsFS())

	// Collected before the file exists, so a bad template creates nothing.
	files, err := appbuild.CollectProjectConfig(fsys, templateDir)
	if err != nil {
		return err
	}
	if _, ok := files["schema.yaml"]; !ok {
		return fmt.Errorf("%s has no schema.yaml to start from", templateDir)
	}

	defer func() {
		if err != nil {
			removeDatabaseFiles(dest)
		}
	}()
	if hasMarkdownData(templateDir) {
		rep, importErr := appbuild.ImportMarkdownData(ctx, fsys, paths, templateDir,
			appbuild.DataImportOptions{Audit: desktopAudit})
		if importErr != nil {
			err = importError(rep, importErr)
			return err
		}
	}
	_, err = appbuild.StoreProjectConfig(ctx, paths, files,
		appbuild.ConfigImportOptions{Source: templateDir, Audit: desktopAudit})
	return err
}

// storeDocumentConfig stores one config file inside the document at path.
func storeDocumentConfig(ctx context.Context, path, name string, content []byte) error {
	paths, err := documentContext(path)
	if err != nil {
		return err
	}
	return appbuild.PutProjectConfigFile(ctx, paths, name, content)
}

// hasMarkdownData reports whether dir holds entities to import.
func hasMarkdownData(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "entities"))
	return err == nil && info.IsDir()
}

// removeDatabaseFiles deletes a database this process just created, with
// SQLite's companion files.
func removeDatabaseFiles(path string) {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + suffix)
	}
}

// addDocumentMenu adds File > New from Template.
func (m *menuBar) addDocumentMenu(file *application.Menu) {
	d := m.d
	file.Add("New from Template…").SetAccelerator("CmdOrCtrl+shift+n").OnClick(func(*application.Context) {
		go d.newFromTemplate()
	})
}

// newFromTemplate asks for a template folder and where to save, creates the
// document and opens it.
// coverage-ignore-func: requires Wails dialogs
func (d *Desktop) newFromTemplate() {
	if d.wails == nil {
		return
	}
	dir, err := d.pickDirectory("Choose a template: a folder with schema.yaml and the other config files", "")
	if err != nil || dir == "" {
		return
	}
	dest, err := d.wails.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:                "New Rela Document",
		Filename:             filepath.Base(dir) + documentExt,
		CanCreateDirectories: true,
		Filters:              []application.FileFilter{{DisplayName: "Rela document", Pattern: "*" + documentExt}},
	}).PromptForSingleSelection()
	if err != nil || dest == "" {
		return
	}
	if !strings.EqualFold(filepath.Ext(dest), documentExt) {
		dest += documentExt
	}
	if err := newDocumentFromTemplate(context.Background(), dir, dest); err != nil {
		d.errorDialog("Could not create the document", err.Error())
		return
	}
	if errMsg := d.LoadProject(dest); errMsg != "" {
		d.errorDialog("Failed to open the document", errMsg)
		return
	}
	d.reloadWindow()
}
