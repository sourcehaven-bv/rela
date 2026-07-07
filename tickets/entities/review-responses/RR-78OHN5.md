---
id: RR-78OHN5
type: review-response
title: Feed output must use stdout capture (ExecuteDocument seam), not a Lua `return` value
finding: |-
    The plan's format-selection section says the handler 'takes the script's returned string' from `return rela.calendar.render(...)`. This contradicts the established script-output mechanism. Verified in internal/script/executor.go: the document-rendering path `ExecuteDocument` (line 83) captures the script's STDOUT into a caller-supplied `io.Writer` — that captured output IS the rendered artifact. It is a deliberately typed seam (comment lines 95-97: 'intentionally NOT taking variadic lua.Option so callers cannot inject arbitrary opts'). There is no return-value plumbing for whole-document output.

    FIX: add a parallel typed seam `ExecuteFeed(path, deps, stdout, feedName, format, timeout)` mirroring ExecuteDocument, and have the feed script emit its rendered output to stdout (the `rela.calendar.render(...)` result is `print()`ed, or the render binding writes to stdout). The handler captures stdout, not a return value. This keeps the feed path consistent with documents and preserves the 'no arbitrary opts' safety property.
severity: significant
resolution: 'Plan updated (PLAN-6LOL0Z §3 + file list): output is captured via stdout through a new typed seam Engine.ExecuteFeed(path, deps, stdout, feedName, format, timeout) mirroring ExecuteDocument (no variadic opts), not a Lua return value. The feed script emits its rendered output to stdout; the handler captures it.'
reason: Superseded by the provider-interface decision (provider-contract.md, PLAN-6LOL0Z §2). The finding assumed the feed script emits a rendered blob, requiring an ExecuteDocument-style stdout seam. The chosen framework/provider shape has the script RETURN a table `{meta?, list, get}` (like ExecuteAction) instead of emitting bytes — rela owns rendering. So there is no stdout capture to get right; the underlying concern (don't invent a wrong output mechanism) is resolved by not having a blob output at all.
status: wont-fix
---
