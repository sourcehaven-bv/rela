package dataentry

import "time"

// toDocumentRenderConfig converts the YAML-facing DocumentConfig into
// the internal render config. configID is the key under `documents:` in
// data-entry.yaml — propagated into the render config so it participates
// in the singleflight/cache key and is exposed to Lua scripts as
// rela.document.id.
//
// Timeout: YAML 0 (unset) is preserved as a zero time.Duration. Each
// renderer chooses its own default — executeCommand clamps to 30s,
// script.Engine.ExecuteDocument delegates to lua.DefaultTimeout. Keeping
// the default in one place per renderer prevents silent drift.
func (a *App) toDocumentRenderConfig(configID string, cfg *DocumentConfig) documentRenderConfig {
	return documentRenderConfig{
		ConfigID: configID,
		Command:  cfg.Command,
		Script:   cfg.Script,
		Timeout:  time.Duration(cfg.Timeout) * time.Second,
	}
}
