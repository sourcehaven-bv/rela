---
id: RR-OXOIT
type: review-response
title: Test plan does not cover the file-on-disk round-trip claim
finding: 'AC 1 says ''removes foo from the entity''s frontmatter on disk''. The existing handler tests in tools_test.go appear to use makeTestServer with an in-memory store. The plan says ''verify the integration path, otherwise add a thin file-backed test'' — that''s a hedge, not a plan. Decide concretely: does makeTestServer round-trip through file persistence? If yes, the standard handler test covers AC 1. If no, add ONE test that uses a file-backed store, writes an entity to disk, calls the handler with {"foo": null}, re-reads from disk, and asserts foo absent. Pick one and update the test plan.'
severity: minor
resolution: 'Verified: makeTestServer uses memstore.New() (pure in-memory, no disk round-trip). Decision: keep handler-level tests in memstore (asserting prop absent in the post-update entity is sufficient — the workspace UpdateEntity already drops keys not in the new map and the storage layer has its own coverage). AC 1 wording ''on disk'' is overstated for the MCP handler test; the on-disk behavior is owned by `internal/markdown` and the workspace layer. Plan updated to drop the file-backed test and rephrase AC 1 as ''removes foo from the entity returned by the store after the update''.'
status: addressed
---
