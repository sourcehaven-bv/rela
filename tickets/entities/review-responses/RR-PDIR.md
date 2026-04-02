---
id: RR-PDIR
type: review-response
title: Missing circular import consideration
finding: |-
    The plan proposes adding workspace reference to the automation Engine. However, there may be a circular import issue:

    - `internal/automation` would import `internal/lua` (for Lua runtime)
    - `internal/lua` imports `internal/workspace` (already)
    - `internal/workspace` imports `internal/automation` (already)

    This could create: `automation -> lua -> workspace -> automation`

    **Recommendation:** Verify import graph before implementing. May need:
    1. Interface-based dependency injection
    2. Move Lua execution to workspace layer
    3. Create a shared interface package
severity: significant
resolution: Created WorkspaceInterface in lua package to break circular import. Workspace implements the interface and is injected into Lua runtime.
status: addressed
---
