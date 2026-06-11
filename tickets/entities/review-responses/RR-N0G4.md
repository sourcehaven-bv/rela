---
id: RR-N0G4
type: review-response
title: 'Cranky #2: empty ProjectRoot makes lua_list silently walk CWD'
finding: 'tools_lua.go handleLuaList: with an empty ProjectRoot, filepath.Join("", "scripts") yields the relative path "scripts" and WalkDir then walks $CWD/scripts, silently listing the wrong directory (errors swallowed via SkipDir). Silent-wrong-answer class. Unreachable in production wiring (ProjectRoot always populated) but a landmine for hand-built Deps{} literals.'
severity: minor
resolution: Added Deps.validate() in internal/mcp/server.go, called from NewServer after the Principal gate; it rejects an empty ProjectRoot (and every nil collaborator). The silent-CWD-walk path in handleLuaList is now unreachable through construction. Covered by TestNewServer_RejectsIncompleteDeps (9 subtests).
reason: 'Fixed at the root: NewServer now calls Deps.validate() which rejects an empty ProjectRoot (plus every nil collaborator) at construction. The silent-CWD-walk path is unreachable through construction. Added TestNewServer_RejectsIncompleteDeps covering all 9 required fields.'
status: addressed
---
