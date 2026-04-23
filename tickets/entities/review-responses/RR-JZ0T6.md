---
id: RR-JZ0T6
type: review-response
title: rela.url wired into every writer runtime; script→frontendroutes edge is wrong layering
finding: 'script.NewWriterRuntime at internal/script/runtime.go:29 unconditionally appends lua.WithRouteCatalog(lua.RouteCatalogFunc(frontendroutes.Has)), exposing rela.url in CLI scripts, scheduler jobs, MCP lua_run, automations, and actions. Those contexts have no frontend. Two real problems: (1) a script that calls rela.url in a non-browser context gets a string that''s never served — silent wrong-thing; (2) a validation rule that hits an unknown path raises a Lua error and fails validation for reasons unrelated to validation. Layering smell too: internal/script is a generic script-execution layer, pulling in Vue-router concepts widens its charter. Parallels WithDocumentMode which is deliberately scoped. Fix: drop WithRouteCatalog from script.NewWriterRuntime; wire it at the document render call site (where script.Engine.ExecuteDocument is invoked) via the opts passthrough. Remove the script→frontendroutes edge from .go-arch-lint.yml.'
severity: significant
resolution: Wiring moved from script.NewWriterRuntime to script.Engine itself (new WithRouteCatalog EngineOption), and applied only inside ExecuteDocument. cmd/rela-server and cmd/rela-desktop construct the engine with script.WithRouteCatalog(lua.RouteCatalogFunc(frontendroutes.Has)); internal/cli callers (scheduler, mcp, flow, root, script) construct bare engines so rela.url is absent in CLI scripts, scheduler jobs, MCP lua_run, actions, and validation. Dropped script->frontendroutes edge from .go-arch-lint.yml; added cmdServer->lua and cmdDesktop->{frontendroutes,lua} edges. just arch-lint clean.
status: addressed
---
