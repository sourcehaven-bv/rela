package dataentry

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/git"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// gitHandler owns the git status and sync routes and the background fetch.
// Extracted from App to take that cluster off the god-object.
//
// The collaborators are closures over App rather than captured values: tests
// and the Configure wiring reassign App.gitOps, App.acl, App.broker and
// App.visibleReader after construction, and a captured copy would go stale
// (the attachmentHandler idiom).
type gitHandler struct {
	// ops is the git working tree; nil when git is not configured.
	ops func() *git.Ops
	// schema is the reloadable snapshot. Each request takes it once.
	schema func() *Schema
	acl    func() acl.ACL
	// broker tells browsers the git status changed.
	broker func() *eventBroker
	// conflictPaths keeps the conflicting files whose rows the request's
	// principal may read; see readableConflictPaths.
	conflictPaths func(ctx context.Context, meta *metamodel.Metamodel, files []string) ([]string, error)
}

// newGitHandler wires the handler to app's live collaborators.
func newGitHandler(app *App) *gitHandler {
	return &gitHandler{
		ops:    func() *git.Ops { return app.gitOps },
		schema: app.State,
		acl:    func() acl.ACL { return app.acl },
		broker: func() *eventBroker { return app.broker },
		conflictPaths: func(ctx context.Context, meta *metamodel.Metamodel, files []string) ([]string, error) {
			return readableConflictPaths(ctx, app.visibleReader, app.paths, meta, files)
		},
	}
}

// GitStatusResponse is the JSON response for /api/git/status.
type GitStatusResponse struct {
	Available     bool     `json:"available"`
	Branch        string   `json:"branch,omitempty"`
	LocalChanges  int      `json:"local_changes"`
	RemoteAhead   int      `json:"remote_ahead"`
	Syncing       bool     `json:"syncing"`
	Conflict      bool     `json:"conflict"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
	// CanSync tells the SPA whether POST /api/v1/_git/sync would be
	// accepted, so it can leave out a sync button that would get a 403.
	CanSync bool `json:"can_sync"`
}

// GitSyncResponse is the JSON response for /api/git/sync.
type GitSyncResponse struct {
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
}

// permitsGitSync reports whether the request's principal may commit and
// push the project. A sync commits every pending change in the working tree,
// so under a policy it needs [acl.PermGitSync]; a read-only instance never
// syncs. The conflict routes use the same check, because resolving a merge
// conflict is part of a sync. It takes the ACL rather than *App so the write
// handler asks the same question.
//
//archplan:guard=authz reason="checks the git:sync permission; read-only refuses"
func permitsGitSync(impl acl.ACL, r *http.Request) bool {
	ctx := r.Context()
	return access.ModeOf(impl).PermitsProjectWrite(ctx, readGateFromContext(ctx), acl.PermGitSync)
}

// writeGitSyncForbidden writes the 403 for a caller without git:sync. It
// names the permission: a sync is a capability of the deployment, not an
// entity, so there is nothing to hide behind a 404. The problem type is
// permission_required, so the SPA can tell it from a write-check 403.
func writeGitSyncForbidden(w http.ResponseWriter, r *http.Request, action string) {
	writeV1Error(w, r, http.StatusForbidden, "permission_required", "Forbidden",
		action+" needs the "+acl.PermGitSync+" permission.")
}

// gitStatusResponse builds the status answer. Only a principal who may sync,
// and so resolve a conflict, gets conflicting file paths; everyone else sees
// that a conflict exists. Of those paths, it lists only the ones the
// conflict list lists: a file naming a row the principal may not read is
// left out (BUG-197XPAU). A gate failure is returned, never folded into
// "no conflict files".
func (h *gitHandler) gitStatusResponse(
	ctx context.Context, status *git.Status, canSync bool,
) (GitStatusResponse, error) {
	resp := GitStatusResponse{
		Available:    status.Available,
		Branch:       status.Branch,
		LocalChanges: status.LocalChanges,
		RemoteAhead:  status.RemoteAhead,
		Syncing:      status.Syncing,
		Conflict:     status.Conflict,
		CanSync:      canSync,
	}
	if !canSync || len(status.ConflictFiles) == 0 {
		return resp, nil
	}
	files, err := h.conflictPaths(ctx, h.schema().Meta, status.ConflictFiles)
	if err != nil {
		return GitStatusResponse{}, err
	}
	resp.ConflictFiles = files
	return resp, nil
}

//archplan:auth=required
func (h *gitHandler) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := GitStatusResponse{Available: false}
	gitOps := h.ops()

	if gitOps == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	status, err := gitOps.GetStatus()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp, err = h.gitStatusResponse(r.Context(), status, permitsGitSync(h.acl(), r))
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGitSync commits the pending changes and pushes them. A caller
// without [acl.PermGitSync] gets the 403 of [writeGitSyncForbidden].
//
//archplan:auth=authz
func (h *gitHandler) handleGitSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !permitsGitSync(h.acl(), r) {
		writeGitSyncForbidden(w, r, "Syncing with git")
		return
	}

	resp := GitSyncResponse{}
	gitOps := h.ops()

	if gitOps == nil {
		resp.Error = "git not configured"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Analyze changes and generate commit message
	changes, err := gitOps.AnalyzeChanges()
	if err != nil {
		resp.Error = "failed to analyze changes: " + err.Error()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	message := changes.GenerateCommitMessage()

	// Perform sync
	if err := gitOps.Sync(message); err != nil {
		resp.Error = err.Error()
		// Check if it's a conflict
		status, _ := gitOps.GetStatus()
		if status != nil && status.Conflict {
			// The same paths the status lists (BUG-197XPAU).
			files, gerr := h.conflictPaths(r.Context(), h.schema().Meta, status.ConflictFiles)
			if gerr != nil {
				writeGateError(w, r, gerr)
				return
			}
			resp.ConflictFiles = files
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Success = true

	// Broadcast git status update
	h.broker().broadcastGitStatus()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// startFetch begins periodic git fetch in the background and returns a stop
// function. It is a no-op when git is not configured or the fetch interval
// is zero.
//
// coverage-ignore-func: background goroutine with timer
func (h *gitHandler) startFetch() (stop func()) {
	gitOps := h.ops()
	cfg := h.schema().Cfg.Git

	if gitOps == nil || cfg == nil || cfg.FetchInterval <= 0 {
		return func() {} // no-op if git not configured or fetch disabled
	}

	// coverage-ignore-start: os-fs-event: background goroutine driven by a real time.Ticker running git fetch (project
	// marks it coverage-ignore)
	interval := time.Duration(cfg.FetchInterval) * time.Second
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := gitOps.Fetch(); err != nil {
					slog.Warn("git fetch error", "error", err)
				} else {
					// Broadcast git status update so UI can refresh
					h.broker().broadcast("git")
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	slog.Info("git background fetch started", "interval", interval)
	return func() {
		close(done)
	}
	// coverage-ignore-end
}
