---
id: RR-R4CHJY
type: review-response
title: Module frames collide with entry-script chunk names
finding: 'Entry scripts compile under a chunk name relative to their own area (actions: bare name; documents and lua_run: relative to scripts/) and each surface remaps frames by string equality. A module compiled as lib/x.lua collides with an entry at scripts/lib/x.lua or actions/lib/x.lua, so module frames get rewritten to the wrong file.'
severity: significant
resolution: Entry scripts now run under their project-relative chunk name via the new Runtime.RunChunk (actions/a.lua, scripts/s.lua) on every surface (action, automation, document, MCP lua_run); the per-surface frame-remap loops are deleted. Pinned by TestRequire_EntryScriptUnderLibSubdirKeepsItsFrames (internal/script).
status: addressed
---
