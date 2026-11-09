package cli

// RemoteCmd groups the commands for the remotes in remotes.yaml
// (FEAT-CMVZQFI).
type RemoteCmd struct {
	Pull RemotePullCmd `cmd:"" help:"Pull the rows a remote in remotes.yaml maps into this project."`
}
