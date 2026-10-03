//go:build sqlite

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// hasProjectDatabase reports whether dir holds a project database. A project
// whose config lives in its database has no schema file on disk, so this is
// what makes such a directory recognizable as a project.
func hasProjectDatabase(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	info, err := os.Lstat(appbuild.DatabasePath(&project.Context{CacheDir: filepath.Join(abs, project.CacheDir)}))
	return err == nil && info.Mode().IsRegular()
}

// databaseOp is one import or export against the active project's database.
// It returns a one-line summary for the user.
type databaseOp func(ctx context.Context, fsys storage.FS, paths *project.Context) (string, error)

// withProjectReleased runs op with the active project closed, then opens it
// again. The database admits one handle at a time, so the project's own
// services must let go of it first; reopening also picks up whatever op
// changed.
func (d *Desktop) withProjectReleased(op databaseOp) (string, error) {
	d.mu.RLock()
	app := d.app
	d.mu.RUnlock()
	if app == nil {
		return "", errors.New("no project is open")
	}
	root := app.ProjectRoot()
	fsys, paths, err := discoverProject(root)
	if err != nil {
		return "", err
	}
	d.releaseLoadedProject()
	summary, opErr := op(context.Background(), fsys, paths)
	if errMsg := d.loadProject(root, false); errMsg != "" {
		opErr = errors.Join(opErr, fmt.Errorf("reopening the project failed: %s", errMsg))
	}
	return summary, opErr
}

// importConfig stores the config files in dir in the project's database,
// replacing the config it carried.
func importConfig(dir string) databaseOp {
	return func(ctx context.Context, fsys storage.FS, paths *project.Context) (string, error) {
		names, err := appbuild.LoadProjectConfig(ctx, fsys, paths, dir)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Stored %d config files in the database.", len(names)), nil
	}
}

// exportConfig writes the config the database carries into dir, refusing to
// overwrite a file that is already there.
func exportConfig(dir string) databaseOp {
	return func(ctx context.Context, fsys storage.FS, paths *project.Context) (string, error) {
		names, err := appbuild.DumpProjectConfig(ctx, fsys, paths, dir, false)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Wrote %d config files to %s.", len(names), dir), nil
	}
}

// importData copies the markdown data in dir into the project's database,
// which must hold no entities yet.
func importData(dir string) databaseOp {
	return func(ctx context.Context, fsys storage.FS, paths *project.Context) (string, error) {
		sink, err := audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))
		if err != nil {
			return "", err
		}
		sum, err := appbuild.ImportMarkdownData(ctx, fsys, paths, dir, appbuild.DataImportOptions{Audit: sink})
		if err != nil {
			return "", err
		}
		return "Imported " + sum.String() + ".", nil
	}
}

// exportData writes the database's data as a markdown project into dir.
func exportData(dir string) databaseOp {
	return func(ctx context.Context, fsys storage.FS, paths *project.Context) (string, error) {
		sum, err := appbuild.ExportMarkdownData(ctx, fsys, paths, dir)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Exported %s to %s.", sum, dir), nil
	}
}

// addDatabaseMenu adds the import and export items for the project database.
func (d *Desktop) addDatabaseMenu(fileMenu *application.Menu) {
	m := fileMenu.AddSubmenu("Project Database")
	m.Add("Import Config from Folder...").OnClick(d.databaseMenuAction(
		"Import Config", "Choose the folder holding schema.yaml and the other config files", importConfig))
	m.Add("Export Config to Folder...").OnClick(d.databaseMenuAction(
		"Export Config", "Choose an empty folder for the config files", exportConfig))
	m.AddSeparator()
	m.Add("Import Markdown Data from Folder...").OnClick(d.databaseMenuAction(
		"Import Data", "Choose the markdown project to import entities, relations and attachments from", importData))
	m.Add("Export Data as Markdown to Folder...").OnClick(d.databaseMenuAction(
		"Export Data", "Choose a folder without entities/, relations/ or attachments/", exportData))
	fileMenu.AddSeparator()
}

// databaseMenuAction asks for a folder, runs the op built for it and reports
// the outcome in a dialog.
// coverage-ignore-func: menu callback - requires Wails runtime
func (d *Desktop) databaseMenuAction(
	title, prompt string, build func(dir string) databaseOp,
) func(*application.Context) {
	return func(*application.Context) {
		d.mu.RLock()
		loaded := d.app != nil
		d.mu.RUnlock()
		if !loaded {
			d.errorDialog(title, "Open a project first.")
			return
		}
		if d.wails == nil {
			return
		}
		// Folders only, and creatable: an export wants a fresh one.
		dir, err := d.wails.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
			Title:                prompt,
			CanChooseDirectories: true,
			CanCreateDirectories: true,
		}).PromptForSingleSelection()
		if err != nil || dir == "" {
			return
		}
		summary, err := d.withProjectReleased(build(dir))
		d.reloadWindow()
		if err != nil {
			d.errorDialog(title+" failed", strings.TrimSpace(err.Error()))
			return
		}
		d.wails.Dialog.Info().SetTitle(title).SetMessage(summary).Show()
	}
}
