package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/git"
)

// gitSyncRepo makes a repository with one commit, an origin remote that does
// not exist and an uncommitted entity file, and returns its path.
func gitSyncRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o600))
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@local", When: time.Now()},
	})
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gogitconfig.RemoteConfig{
		Name: "origin", URLs: []string{filepath.Join(t.TempDir(), "missing")},
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "entities", "ticket"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "entities", "ticket", "TKT-1.md"),
		[]byte("---\ntitle: x\n---\n"), 0o600))
	return dir
}

func gitCommitCount(t *testing.T, dir string) int {
	t.Helper()
	repo, err := gogit.PlainOpen(dir)
	require.NoError(t, err)
	iter, err := repo.Log(&gogit.LogOptions{})
	require.NoError(t, err)
	n := 0
	require.NoError(t, iter.ForEach(func(*object.Commit) error { n++; return nil }))
	return n
}

// TestHandleGitSync_Gate pins who may commit and push. A refused sync must
// not commit: the commit count is the evidence that Sync never ran. A sync
// that runs commits the pending entity file and then fails to fetch from the
// missing remote, which still answers 200 with the error in the body.
func TestHandleGitSync_Gate(t *testing.T) {
	cases := []struct {
		name  string
		acl   string // "nop", "read-only", "policy" or "broken"
		holds bool
		want  int
	}{
		{name: "no policy syncs", acl: "nop", want: http.StatusOK},
		{name: "read-only refused", acl: "read-only", holds: true, want: http.StatusForbidden},
		{name: "policy without git:sync refused", acl: "policy", want: http.StatusForbidden},
		{name: "policy with git:sync syncs", acl: "policy", holds: true, want: http.StatusOK},
		{name: "broken policy refused", acl: "broken", holds: true, want: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			switch tc.acl {
			case "nop":
				setTestACL(t, app, acl.NopACL{})
			case "read-only":
				setTestACL(t, app, acl.ReadOnlyACL{})
			case "policy":
				d, err := acl.NewDeclarative(&acl.Policy{}, aclstore.MustNewGraph(app.store), app.store)
				require.NoError(t, err)
				setTestACL(t, app, d)
			case "broken":
				setTestACL(t, app, (*acl.Declarative)(nil))
			}
			dir := gitSyncRepo(t)
			app.gitOps = git.NewOps(dir, git.Config{Enabled: true, Branch: "master"})
			before := gitCommitCount(t, dir)

			ctx := withReadGate(context.Background(), fakeGate{holdsPermission: tc.holds})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/_git/sync", http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			app.handleGitSync(rec, req)

			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			committed := gitCommitCount(t, dir) - before
			if tc.want == http.StatusForbidden {
				require.Zero(t, committed, "a refused sync must not commit")
				require.Contains(t, rec.Body.String(), acl.PermGitSync)
			} else {
				require.Equal(t, 1, committed, "an allowed sync commits the pending entity file")
			}
		})
	}
}

// TestHandleGitStatus_CanSync pins that the status reports whether a sync
// would be accepted, under the same gate as the sync itself, and that the
// status stays readable for everyone.
func TestHandleGitStatus_CanSync(t *testing.T) {
	cases := []struct {
		name  string
		acl   string // "nop", "read-only" or "policy"
		holds bool
		want  bool
	}{
		{name: "no policy", acl: "nop", want: true},
		{name: "read-only", acl: "read-only", holds: true, want: false},
		{name: "policy without git:sync", acl: "policy", want: false},
		{name: "policy with git:sync", acl: "policy", holds: true, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			switch tc.acl {
			case "nop":
				setTestACL(t, app, acl.NopACL{})
			case "read-only":
				setTestACL(t, app, acl.ReadOnlyACL{})
			case "policy":
				d, err := acl.NewDeclarative(&acl.Policy{}, aclstore.MustNewGraph(app.store), app.store)
				require.NoError(t, err)
				setTestACL(t, app, d)
			}
			app.gitOps = git.NewOps(gitSyncRepo(t), git.Config{Enabled: true, Branch: "master"})

			ctx := withReadGate(context.Background(), fakeGate{holdsPermission: tc.holds})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_git/status", http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			app.handleGitStatus(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			var resp GitStatusResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.True(t, resp.Available)
			require.Equal(t, tc.want, resp.CanSync)
		})
	}
}

// TestGitStatusResponse_ConflictFiles: only a principal who may sync gets the
// conflicting paths; everyone sees that a conflict exists.
func TestGitStatusResponse_ConflictFiles(t *testing.T) {
	status := &git.Status{Available: true, Conflict: true, ConflictFiles: []string{"entities/ticket/TKT-9.md"}}

	hidden := gitStatusResponse(status, false)
	require.True(t, hidden.Conflict)
	require.Empty(t, hidden.ConflictFiles)

	shown := gitStatusResponse(status, true)
	require.Equal(t, status.ConflictFiles, shown.ConflictFiles)
}
