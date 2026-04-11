package dataentry

import (
	"time"

	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// toWorkspaceDocConfig converts dataentry config to workspace config.
func (a *App) toWorkspaceDocConfig(cfg *DocumentConfig) workspace.DocumentConfig {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return workspace.DocumentConfig{
		Command: cfg.Command,
		Timeout: timeout,
	}
}
