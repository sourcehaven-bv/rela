---
id: RR-BJ2ZWT
type: review-response
title: Subscription path allowlist admits underscore routes
finding: The /api/v1/{plural}/{id} pattern also matches _export, _documents, _history, _comments; re-running those per change would run Lua or external commands as a DoS amplifier.
severity: significant
resolution: Paths are parsed into typed parts (plural from the schema, safe id not starting with _, single-segment view ids, clean path, per-shape query-key allowlist) and the replay URL is rebuilt from them. Negative tests listed.
status: addressed
---
