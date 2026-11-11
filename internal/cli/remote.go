package cli

// RemoteCmd groups the commands for the remotes in remotes.yaml
// (FEAT-CMVZQFI).
type RemoteCmd struct {
	Pull      RemotePullCmd      `cmd:"" help:"Sync a remote in remotes.yaml: pull the rows it maps, and push local changes when it has push."`
	Conflicts RemoteConflictsCmd `cmd:"" help:"List the conflicts a remote holds for a person to resolve."`
	Resolve   RemoteResolveCmd   `cmd:"" help:"Resolve a held conflict to one side and sync, or dismiss what the latest rule left."`
}
