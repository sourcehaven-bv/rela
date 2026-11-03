package dataentry

import (
	"encoding/json"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/access"
	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/git"
)

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
// and so resolve a conflict, gets the conflicting file paths; everyone else
// sees that a conflict exists. This is not a secrecy guarantee: a holder of
// git:sync sees the paths of entities they may not read. Conflicts arise only
// on file storage, which runs locally, so the coarse gate is accepted
// (BUG-T6HNKS).
func gitStatusResponse(status *git.Status, canSync bool) GitStatusResponse {
	resp := GitStatusResponse{
		Available:    status.Available,
		Branch:       status.Branch,
		LocalChanges: status.LocalChanges,
		RemoteAhead:  status.RemoteAhead,
		Syncing:      status.Syncing,
		Conflict:     status.Conflict,
		CanSync:      canSync,
	}
	if canSync {
		resp.ConflictFiles = status.ConflictFiles
	}
	return resp
}

//archplan:auth=required
func (a *App) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := GitStatusResponse{Available: false}
	gitOps := a.gitOps

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

	resp = gitStatusResponse(status, permitsGitSync(a.acl, r))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGitSync commits the pending changes and pushes them. A caller
// without [acl.PermGitSync] gets the 403 of [writeGitSyncForbidden].
//
//archplan:auth=authz
func (a *App) handleGitSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !permitsGitSync(a.acl, r) {
		writeGitSyncForbidden(w, r, "Syncing with git")
		return
	}

	resp := GitSyncResponse{}
	gitOps := a.gitOps

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
			resp.ConflictFiles = status.ConflictFiles
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Success = true

	// Broadcast git status update
	a.broker.broadcastGitStatus()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
