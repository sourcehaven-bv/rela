---
id: RR-UXZAE
type: review-response
title: ExecuteAction missing WithCache wiring (round 2)
finding: script.Engine.ExecuteAction constructs the runtime without lua.WithCache(e.cache), so rela.cache.* inside action scripts errors with 'attempt to index a nil value'. The data-entry server's long-lived engine holds a cache but actions — the primary consumer — couldn't see it. Also needed SetScriptPath since RunActionString doesn't infer it.
severity: significant
resolution: Added lua.WithCache(e.cache) to the NewWriterRuntime options in script/action.go ExecuteAction. Also added runtime.SetScriptPath(scriptPath) before RunActionString because that entry point doesn't set it automatically. Actions can now use rela.cache.* and share state with other engine-built runtimes.
status: addressed
---
