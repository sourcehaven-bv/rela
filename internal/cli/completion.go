package cli

import "fmt"

// CompletionCmd generates shell completion scripts.
//
// Note: kong does not ship shell-completion generation in core. The
// previous cobra implementation called rootCmd.GenXxxCompletion; for
// kong we surface a not-implemented error pending integration of a
// completion package (e.g. github.com/willabides/kongplete).
type CompletionCmd struct {
	Shell string `arg:"" enum:"bash,zsh,fish,powershell" help:"Shell to generate completion for (bash|zsh|fish|powershell)."`
}

// Run executes `rela completion <shell>`.
func (c *CompletionCmd) Run() error {
	return fmt.Errorf("completion for %s is not implemented in the kong CLI yet", c.Shell)
}
