---
id: RR-FLCXC
type: review-response
title: Handler must enforce DocumentConfig.EntityType matches entity.Type
finding: handleV1Documents does not check that the rendered entity's type matches DocumentConfig.EntityType. An HTTP caller can invoke /documents/release_notes/TKT-CGBVW and run release-notes.lua against a ticket entity. Script authors write to an assumed type; mismatch can drive unexpected code paths and — worse — exfiltrate data via ai.chat the author thought was scoped to releases.
severity: critical
resolution: Handler verifies entity.Type == docCfg.EntityType before render; mismatch returns HTTP 400 (plan approach §5). Verified in AC9 + api_v1_test.go.
status: addressed
---

From design-review on PLAN-78HJO.

Belongs at the HTTP handler layer (`api_v1.go` / `handlers_document.go`), not
inside Lua — authors shouldn't have to defensively type-check. Existing
`command:` path has the same hole with lower blast radius.

Fix: before Render, load the entity, verify `entity.Type == docCfg.EntityType`,
return HTTP 400/404 on mismatch.
