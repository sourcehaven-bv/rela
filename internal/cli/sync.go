package cli

import (
	"context"

	syncclient "github.com/Sourcehaven-BV/rela/internal/cli/sync"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
)

// SyncCmd is the `rela sync` command group: push local changes to and pull
// remote changes from a proxy-fronted rela-server's /api/sync/ API. See
// internal/cli/sync for the engine and TKT-T4H4YK for the design.
type SyncCmd struct {
	Push SyncPushCmd `cmd:"" help:"Push locally-changed records to the remote rela-server."`
	Pull SyncPullCmd `cmd:"" help:"Pull remote changes into the local project."`
}

// syncCommon holds the flags shared by push and pull. Remote is the
// proxy-fronted base URL; Token is the bearer presented to the proxy (env
// RELA_SYNC_TOKEN preferred so it never lands in shell history). Token is read
// but NEVER printed or logged.
type syncCommon struct {
	Remote string `help:"Remote rela-server base URL (proxy-fronted)." env:"RELA_REMOTE"`
	Token  string `help:"Bearer token for the OAuth proxy (prefer the RELA_SYNC_TOKEN env var)." env:"RELA_SYNC_TOKEN"`
}

// SyncPushCmd pushes local changes. With Force set, it resolves a single record
// in favor of the local copy (local wins) and re-baselines the index.
type SyncPushCmd struct {
	syncCommon
	Force string `help:"Resolve a conflict for one record id: overwrite the remote with the local copy."`
}

// SyncPullCmd pulls remote changes. With Force set, it resolves a single record
// in favor of the remote copy (remote wins) and re-baselines the index.
type SyncPullCmd struct {
	syncCommon
	Force string `help:"Resolve a conflict for one record id: overwrite the local copy with the remote."`
}

// Run executes `rela sync push`.
func (c *SyncPushCmd) Run(ctx context.Context, svc *cliServices) error {
	eng, idx, cacheDir, err := buildSyncEngine(c.Remote, c.Token, svc)
	if err != nil {
		return err
	}

	if c.Force != "" {
		res, ferr := eng.ForcePush(ctx, c.Force)
		if ferr != nil {
			return ferr
		}
		if serr := idx.Save(svc.FS(), cacheDir); serr != nil {
			return serr
		}
		out.WriteSuccess("force-pushed %s (local wins)", res.Key)
		return nil
	}

	report, err := eng.Push(ctx)
	// Always persist whatever progress was made before reporting an error.
	if serr := idx.Save(svc.FS(), cacheDir); serr != nil && err == nil {
		err = serr
	}
	if err != nil {
		return err
	}
	return reportPush(report)
}

// Run executes `rela sync pull`.
func (c *SyncPullCmd) Run(ctx context.Context, svc *cliServices) error {
	eng, idx, cacheDir, err := buildSyncEngine(c.Remote, c.Token, svc)
	if err != nil {
		return err
	}

	if c.Force != "" {
		res, ferr := eng.ForcePull(ctx, c.Force)
		if ferr != nil {
			return ferr
		}
		if serr := idx.Save(svc.FS(), cacheDir); serr != nil {
			return serr
		}
		out.WriteSuccess("force-pulled %s (remote wins)", res.Key)
		return nil
	}

	report, err := eng.Pull(ctx)
	if serr := idx.Save(svc.FS(), cacheDir); serr != nil && err == nil {
		err = serr
	}
	if err != nil {
		return err
	}
	return reportPull(report)
}

// buildSyncEngine wires the sync engine from the CLI services: it loads the
// index, constructs the HTTP client, and type-asserts the entity manager to the
// id-preserving applier (the same consumer-side pattern as the server — these
// methods are intentionally off the broad EntityManager interface). Returns the
// engine, the index (so the caller can Save it), and the cache dir.
func buildSyncEngine(remote, token string, svc *cliServices) (*syncclient.Engine, *syncclient.State, string, error) {
	client, err := syncclient.NewClient(remote, token, nil)
	if err != nil {
		return nil, nil, "", err
	}
	cacheDir := svc.Paths().CacheDir
	idx, err := syncclient.LoadState(svc.FS(), cacheDir)
	if err != nil {
		return nil, nil, "", err
	}
	applier, ok := svc.EntityManager().(syncclient.LocalApplier)
	if !ok {
		// fs/memory builds use *entitymanager.Manager, which satisfies this; a
		// build that doesn't would only break pull (push needs no applier).
		applier = nil
	}
	eng, err := syncclient.NewEngine(client, svc.Store(), applier, idx)
	if err != nil {
		return nil, nil, "", err
	}
	return eng, idx, cacheDir, nil
}

// reportPush prints a push report and returns a non-zero exit error if any
// record halted on a conflict or validation failure, so scripts can detect an
// unconverged push.
func reportPush(r *syncclient.PushReport) error {
	for _, res := range r.Results {
		switch res.Outcome {
		case syncclient.OutcomePushed:
			out.WriteMessage("pushed   %s", res.Key)
		case syncclient.OutcomeDeleted:
			out.WriteMessage("deleted  %s", res.Key)
		case syncclient.OutcomeConflict:
			out.WriteWarning("CONFLICT %s — %s", res.Key, res.Detail)
		case syncclient.OutcomeInvalid:
			out.WriteWarning("INVALID  %s — %s", res.Key, res.Detail)
		}
	}
	if r.Locked > 0 {
		out.WriteWarning("%d locked record(s) skipped (unreadable: git-crypt etc.)", r.Locked)
	}
	out.WriteInfo("push: %d applied, %d deleted, %d conflict(s), %d invalid",
		r.Applied, r.Deleted, r.Conflicts, r.Invalid)
	if r.Conflicts > 0 || r.Invalid > 0 {
		return unconvergedError()
	}
	return nil
}

// reportPull prints a pull report and returns a non-zero exit error if any
// record halted on a conflict.
func reportPull(r *syncclient.PullReport) error {
	for _, res := range r.Results {
		switch res.Outcome {
		case syncclient.OutcomePulled:
			out.WriteMessage("pulled   %s", res.Key)
		case syncclient.OutcomePulledDelete:
			out.WriteMessage("deleted  %s", res.Key)
		case syncclient.OutcomePullSkipped:
			// no-op; keep quiet unless verbose
			if verbose {
				out.WriteMessage("in-sync  %s", res.Key)
			}
		case syncclient.OutcomePullConflict:
			out.WriteWarning("CONFLICT %s — %s", res.Key, res.Detail)
		}
	}
	out.WriteInfo("pull: %d applied, %d deleted, %d conflict(s), %d in-sync",
		r.Applied, r.Deleted, r.Conflicts, r.Skipped)
	if r.Conflicts > 0 {
		return unconvergedError()
	}
	return nil
}

// unconvergedError is a quiet exit-1: the report already explained each halt, so
// we don't want kong to print a second error line. runKong special-cases
// ExitError to set the exit code without re-printing.
func unconvergedError() error {
	return &relaerrors.ExitError{Code: 1}
}
