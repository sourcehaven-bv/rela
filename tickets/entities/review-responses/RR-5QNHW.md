---
id: RR-5QNHW
type: review-response
title: Logging %v on error paths risks leaking raw keys
finding: 'The plan promises ''never log raw keys'' and uses `key_hash`. But `set` returns a Lua error on oversized/wrong-type, and `ls.RaiseError(''cache set error: %s'', err)` propagates `err.Error()`. Add an explicit test: set a key with a recognizable substring, trigger every error path, assert the substring appears in no emitted log line or returned error.'
severity: significant
resolution: 'Addressed in AC 15: ''raw script paths and raw keys never logged, never included in any Lua error message (test-enforced)''. Test added to AC 17: ''logs contain namespace_hash/key_hash, never raw path or key''. Implementation note in the plan specifies pattern-matching a recognizable key/path substring against all emitted errors and log lines.'
status: addressed
---
