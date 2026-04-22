---
id: RR-BOV25
type: review-response
title: Timeout default duplicated between handlers_document.go and lua.DefaultTimeout
finding: toDocumentRenderConfig substitutes 30s when cfg.Timeout == 0; lua.DefaultTimeout is also 30s. If lua.DefaultTimeout changes, the data-entry path silently ignores the update. ExecuteDocument already falls back to DefaultTimeout when timeout <= 0 via its guard; the handler-side substitution is redundant and wrong.
severity: significant
resolution: toDocumentRenderConfig no longer substitutes 30s; passes cfg.Timeout as-is (0 when YAML omits it). executeCommand gains its own commandDefaultTimeout const + zero-guard (addresses paired RR-HSEJ6 too). script.Engine.ExecuteDocument already delegates to lua.DefaultTimeout via the `timeout > 0` guard. Default lives once per renderer.
status: addressed
---

From go-architect review finding #11.

Fix: pass cfg.Timeout through as-is (including 0). ExecuteDocument's `if timeout
> 0` guard skips WithTimeout and the runtime uses lua.DefaultTimeout. Single
> source of truth for the default. The command-renderer path already passed
> through the substituted value; keep the 30s default there in executeCommand
> directly.
