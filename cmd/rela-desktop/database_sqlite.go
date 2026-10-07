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
	"github.com/Sourcehaven-BV/rela/internal/fsimport"
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
	root := d.activePath // a document's file, not its workspace
	d.mu.RUnlock()
	if app == nil {
		return "", errors.New("no project is open")
	}
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
		rep, err := appbuild.ImportMarkdownData(ctx, fsys, paths, dir, appbuild.DataImportOptions{Audit: desktopAudit})
		if err != nil {
			return "", importError(rep, err)
		}
		msg := fmt.Sprintf("Imported %d entities, %d relations, %d attachments and %d comments.",
			rep.Entities, rep.Relations, rep.Attachments, rep.Comments)
		if n := len(rep.Skipped); n > 0 {
			msg += fmt.Sprintf(" %d files were not copied.", n)
		}
		return msg, nil
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
func (m *menuBar) addDatabaseMenu(fileMenu *application.Menu) {
	d := m.d
	sub := fileMenu.AddSubmenu("Project Database")
	sub.Add("Import Config from Folder...").OnClick(d.databaseMenuAction(
		"Import Config", "Choose the folder holding schema.yaml and the other config files", importConfig))
	sub.Add("Export Config to Folder...").OnClick(d.databaseMenuAction(
		"Export Config", "Choose an empty folder for the config files", exportConfig))
	sub.AddSeparator()
	sub.Add("Import Markdown Data from Folder...").OnClick(d.databaseMenuAction(
		"Import Data", "Choose the markdown project to import entities, relations and attachments from", importData))
	sub.Add("Export Data as Markdown to Folder...").OnClick(d.databaseMenuAction(
		"Export Data", "Choose a folder without entities/, relations/ or attachments/", exportData))
	fileMenu.AddSeparator()
}

// databaseMenuAction confirms which project the action applies to, asks for
// a folder, runs the op built for it and reports the outcome in a dialog.
//
// The menu is application-wide and acts on the most recently opened project,
// which need not be the focused window's; the confirmation names it so the
// user cannot replace another project's config by accident.
// coverage-ignore-func: menu callback - requires Wails runtime
func (d *Desktop) databaseMenuAction(
	title, prompt string, build func(dir string) databaseOp,
) func(*application.Context) {
	return func(*application.Context) {
		d.mu.RLock()
		app := d.app
		activePath := d.activePath
		d.mu.RUnlock()
		if d.wails == nil {
			return
		}
		if app == nil {
			d.errorDialog(title, "Open a project first.")
			return
		}
		confirm := d.wails.Dialog.Question().SetTitle(title).
			SetMessage(fmt.Sprintf("This applies to %s (%s).", app.ProjectName(), activePath))
		confirm.AddButton("Continue").SetAsDefault().OnClick(func() {
			go d.runDatabaseAction(title, prompt, build)
		})
		confirm.AddButton("Cancel").SetAsCancel()
		confirm.Show()
	}
}

// runDatabaseAction is the part of a database menu action after the user
// confirmed the project.
// coverage-ignore-func: menu callback - requires Wails runtime
func (d *Desktop) runDatabaseAction(title, prompt string, build func(dir string) databaseOp) {
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

// importError adds the import's problems to err, so the dialog says what to
// fix rather than only that the import failed.
func importError(rep *fsimport.Report, err error) error {
	if rep == nil || len(rep.Errors) == 0 {
		return err
	}
	const shown = 10
	list, more := rep.Errors, ""
	if len(list) > shown {
		more = fmt.Sprintf("\n…and %d more", len(list)-shown)
		list = list[:shown]
	}
	return fmt.Errorf("%w:\n- %s%s", err, strings.Join(list, "\n- "), more)
}
