package dataentry

import (
	"encoding/json"
	"net/http"
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
}

// GitSyncResponse is the JSON response for /api/git/sync.
type GitSyncResponse struct {
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
	ConflictFiles []string `json:"conflict_files,omitempty"`
}

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
		resp.Available = false
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Available = status.Available
	resp.Branch = status.Branch
	resp.LocalChanges = status.LocalChanges
	resp.RemoteAhead = status.RemoteAhead
	resp.Syncing = status.Syncing
	resp.Conflict = status.Conflict
	resp.ConflictFiles = status.ConflictFiles

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *App) handleGitSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
