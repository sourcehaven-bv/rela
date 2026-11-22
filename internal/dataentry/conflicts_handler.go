package dataentry

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/conflict"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// conflictsHandler owns the merge-conflict read routes: the list at
// /api/v1/_conflicts and the detail at /api/v1/_conflicts/{path}. The resolve
// write is mounted on the same prefix but lives on writeHandler. Extracted
// from App to take that cluster off the god-object.
//
// The collaborators are closures over App rather than captured values: tests
// reassign App.acl, App.paths, App.visibleReader, App.affordances and
// App.write after construction, and a captured copy would go stale (the
// gitHandler idiom).
type conflictsHandler struct {
	acl    func() acl.ACL
	paths  func() *project.Context
	schema func() *Schema
	// reader row-gates the identities a conflicted file names.
	reader func() visibleReader
	// hiddenProperties lists the properties `visible:` hides on either side
	// of a conflicted file (affordanceService.conflictHiddenProperties).
	hiddenProperties func(ctx context.Context, cf *conflict.ConflictedFile, face entityPkg.Face) map[string]struct{}
	// resolve serves POST /api/v1/_conflicts/resolve.
	resolve http.HandlerFunc
}

// newConflictsHandler wires the handler to app's live collaborators.
func newConflictsHandler(app *App) *conflictsHandler {
	return &conflictsHandler{
		acl:    func() acl.ACL { return app.acl },
		paths:  func() *project.Context { return app.paths },
		schema: app.State,
		reader: func() visibleReader { return app.visibleReader },
		hiddenProperties: func(
			ctx context.Context, cf *conflict.ConflictedFile, face entityPkg.Face,
		) map[string]struct{} {
			return app.affordances.conflictHiddenProperties(ctx, cf, face)
		},
		resolve: func(w http.ResponseWriter, r *http.Request) { app.write.handleV1ConflictResolve(w, r) },
	}
}

// handleV1Conflicts returns the list of conflicted files as JSON. It is
// served only to a principal who may sync (git:sync); everyone else gets a
// 403 (BUG-T6HNKS). The list is also row-gated: a file that names an entity
// or relation the principal may not read is left out (BUG-Q3Z15V; see
// conflictsReadable).
//
//archplan:auth=authz reason="requires git:sync via permitsGitSync, then row-gates each file"
func (h *conflictsHandler) handleV1Conflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if !permitsGitSync(h.acl(), r) {
		writeGitSyncForbidden(w, r, "Listing merge conflicts")
		return
	}

	paths := h.paths()
	ctx := &project.Context{
		Root:         paths.Root,
		EntitiesDir:  paths.EntitiesDir,
		RelationsDir: paths.RelationsDir,
	}

	result, err := conflict.DetectAll(ctx)
	if err != nil {
		writeInternalError(w, r, "conflict_detection_failed", "Failed to detect conflicts", err)
		return
	}

	// Parse each file for the identities it names, then gate them all in
	// one batch. A file that no longer parses as conflicted is left out:
	// resolved since the scan, or unreadable, which the log records.
	meta := h.schema().Meta
	files := make([]conflict.ConflictedFile, 0, len(result.Files))
	ids := make([]conflictIdentity, 0, len(result.Files))
	for _, cf := range result.Files {
		parsed, perr := conflict.ParseConflictedFile(cf.Path, meta)
		if perr != nil {
			if !errors.Is(perr, fs.ErrNotExist) {
				slog.Warn("dataentry: skipping a conflicted file that does not parse", "path", cf.Path, "err", perr)
			}
			continue
		}
		files = append(files, cf)
		ids = append(ids, identifyConflict(paths, meta, parsed))
	}
	readable, err := conflictsReadable(r.Context(), h.reader(), ids)
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	items := make([]v1.ConflictItem, 0, len(files))
	for i, cf := range files {
		if !readable[i] {
			continue
		}
		relPath, _ := filepath.Rel(ctx.Root, cf.Path)
		items = append(items, v1.ConflictItem{
			Path:        relPath,
			EntityType:  cf.EntityType,
			EntityID:    cf.EntityID,
			MarkerCount: len(cf.Markers),
		})
	}

	writeV1JSON(w, http.StatusOK, v1.ConflictsResponse{
		Conflicts: items,
		Count:     len(items),
	})
}

// handleV1ConflictRoutes handles GET /api/v1/_conflicts/{path} and POST /api/v1/_conflicts/resolve.
// Both require git:sync, like the list. The detail checks it here, before the
// path is resolved. It then row-gates the file, answering a hidden one with
// the same 404 as a missing one, and leaves out every property `visible:`
// hides on either side (BUG-Q3Z15V). Resolve is gated in
// [writeHandler.handleV1ConflictResolve].
//
//archplan:auth=authz reason="detail requires git:sync, then row-gates and redacts; resolve is gated in its handler"
func (h *conflictsHandler) handleV1ConflictRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_conflicts/")

	if path == "resolve" && r.Method == http.MethodPost {
		h.resolve(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	if !permitsGitSync(h.acl(), r) {
		writeGitSyncForbidden(w, r, "Viewing a merge conflict")
		return
	}

	// Get conflict details. The path is caller-supplied — contain it to
	// the project root before any filesystem access.
	paths := h.paths()
	absPath, ok := resolveConflictPath(paths, w, r, path)
	if !ok {
		return
	}

	meta := h.schema().Meta
	cf, ok := parseConflictOr404(w, r, absPath, meta)
	if !ok {
		return
	}
	id := identifyConflict(paths, meta, cf)
	if !conflictReadableOr404(w, r, h.reader(), id) {
		return
	}

	info := conflict.AnalyzeConflict(cf)
	hidden := h.hiddenProperties(r.Context(), cf, id.face())

	diffs := make([]v1.PropertyDiff, 0, len(info.PropertyDiffs))
	for _, d := range info.PropertyDiffs {
		if _, ok := hidden[d.Property]; ok {
			continue
		}
		diffs = append(diffs, v1.PropertyDiff{
			Property:    d.Property,
			OursValue:   fmt.Sprintf("%v", d.OursValue),
			TheirsValue: fmt.Sprintf("%v", d.TheirsValue),
			IsSame:      d.IsSame,
		})
	}

	detail := v1.ConflictDetail{
		Path:          path,
		EntityType:    cf.EntityType,
		EntityID:      cf.EntityID,
		PropertyDiffs: diffs,
		ContentSame:   info.ContentSame,
		ContentOurs:   info.ContentDiffOurs,
		ContentTheirs: info.ContentDiffTheirs,
	}

	writeV1JSON(w, http.StatusOK, detail)
}
