package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Downloads.
//
// A browser saves a response sent as an attachment; a webview does nothing
// with it. Exports, attachments and command results are all links to /api/
// URLs, so the page script sends those clicks to Downloads.Save instead
// (see multiWindowScript). The file is fetched through the same handler the page
// uses, so it passes the same checks a browser download would.

// maxDownloadBytes bounds what is held in memory before it is written out.
// Export output is already capped by the transform engine; this covers
// attachments.
const maxDownloadBytes = 512 << 20

// maxErrorChars is how much of a refusal's body the error dialog shows.
const maxErrorChars = 300

// savedFileMode keeps a saved download private to the user, as a browser
// does.
const savedFileMode = 0o600

// Downloads is the download service bound to the page. It is its own type,
// not a Desktop method, so the page reaches it as main.Downloads.Save.
type Downloads struct {
	// handler serves the request, exactly as the page's own fetch would be.
	handler http.Handler
	// baseCtx is the context requests run under; nil means Background.
	baseCtx func() context.Context
	// prompt asks where to save name: the chosen path, or "" on cancel.
	prompt func(name string) (string, error)
	// alert shows an error the page has no place for.
	alert func(title, message string)
}

// newDownloads checks its collaborators; see the struct for each one.
func newDownloads(dl Downloads) (*Downloads, error) {
	if dl.handler == nil || dl.prompt == nil || dl.alert == nil {
		return nil, errors.New("downloads: handler, prompt and alert are required")
	}
	return &dl, nil
}

// Save fetches path from the app and saves it where the user picks. Returns
// an error string, empty on success or when the user cancels. Errors are
// also shown in a dialog.
func (dl *Downloads) Save(path string) string {
	name, body, err := dl.fetch(path)
	if err == nil {
		err = dl.saveAs(name, body)
	}
	if err != nil {
		dl.alert("Download failed", err.Error())
		return err.Error()
	}
	return ""
}

// fetch serves path through the handler and returns the file name the
// response suggests and its body.
func (dl *Downloads) fetch(raw string) (name string, body []byte, err error) {
	route, err := safeRoute(raw)
	if err != nil {
		return "", nil, err
	}
	ctx := context.Background()
	if dl.baseCtx != nil {
		if c := dl.baseCtx(); c != nil {
			ctx = c
		}
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, route, http.NoBody)
	rec := httptest.NewRecorder()
	dl.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		msg := strings.TrimSpace(rec.Body.String())
		if len(msg) > maxErrorChars {
			msg = msg[:maxErrorChars] + "…"
		}
		return "", nil, fmt.Errorf("the server answered %d: %s", rec.Code, msg)
	}
	if rec.Body.Len() > maxDownloadBytes {
		return "", nil, errors.New("the file is too large to save from here")
	}
	return downloadName(rec.Header(), route), rec.Body.Bytes(), nil
}

// downloadName is the file name to offer: the response's attachment name if
// it sent one, else the last segment of the path. Either way only a base
// name survives, so a response cannot suggest a path.
func downloadName(h http.Header, route string) string {
	if _, params, err := mime.ParseMediaType(h.Get("Content-Disposition")); err == nil {
		if name := cleanFileName(params["filename"]); name != "" {
			return name
		}
	}
	p := route
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if name := cleanFileName(path.Base(p)); name != "" {
		return name
	}
	return "download"
}

// cleanFileName reduces name to a plain file name, or "" if nothing usable
// is left.
func cleanFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == ':' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == ".." || name == "_export" {
		return ""
	}
	return name
}

// saveAs asks where to save and writes body there. Canceling is not an
// error.
func (dl *Downloads) saveAs(name string, body []byte) error {
	dest, err := dl.prompt(name)
	if err != nil || dest == "" {
		return nil //nolint:nilerr // a closed panel is a cancel
	}
	if err := os.WriteFile(dest, body, savedFileMode); err != nil {
		return fmt.Errorf("could not write %s: %w", filepath.Base(dest), err)
	}
	slog.Debug("saved download", "file", dest, "bytes", len(body))
	return nil
}

// promptSave is the native save panel for a download.
// coverage-ignore-func: requires Wails runtime
func promptSave(app *application.App) func(name string) (string, error) {
	return func(name string) (string, error) {
		return app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
			Title:                "Save",
			Filename:             name,
			CanCreateDirectories: true,
		}).PromptForSingleSelection()
	}
}
