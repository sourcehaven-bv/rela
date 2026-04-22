---
id: RR-DBCF1
type: review-response
title: Print redirect silently drops automation & scheduler print() output
finding: Unconditional luaPrint override routes print() to r.stdout in every runtime. internal/script/executor.go:130 (Engine.execute) uses a bytes.Buffer that nothing reads, so every print() in an automation/scheduled-task script now lands in a dead-letter buffer. Previously hit os.Stdout via gopher-lua default, so operators could see progress output. Scope the override to (isDocument || isAction) only.
severity: significant
resolution: luaPrint override now scoped to `if r.isDocument || r.isAction` in newRuntime. CLI, scheduler, MCP lua_eval/lua_run, validation, and automation scripts restore the gopher-lua default (writes to os.Stdout). Document-mode context test updated to use rela.output for readback since print() is no longer captured there.
status: addressed
---

From post-impl cranky review (+ go-architect finding #1).

Fix chosen: override luaPrint only when captured stdout is meaningful — document
mode (captured as markdown) and action mode (warning-line channel). CLI,
scheduler, MCP lua_eval/lua_run, validation, and automation scripts restore the
gopher-lua default (writes to os.Stdout) which matches existing user
expectations.
