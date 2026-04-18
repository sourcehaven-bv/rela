package dataentry

import "time"

// toDocumentRenderConfig converts the YAML-facing DocumentConfig into
// the internal render config.
func (a *App) toDocumentRenderConfig(cfg *DocumentConfig) documentRenderConfig {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return documentRenderConfig{
		Command: cfg.Command,
		Timeout: timeout,
	}
}
