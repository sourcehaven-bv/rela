---
id: RR-EHNQ6Y
type: review-response
title: SeenHooks puts lock waits and KV I/O on every rename, in every binary
finding: '§0 rejects a manager hook because it "would put channel I/O on every write path, in every binary including the CLI". `SeenHooks.EntityRenamed` does exactly that: entitymanager calls alias hooks synchronously on the request (alias_hook.go, up to 30s). Each rename waits up to SeenLockWait 2s for the round lock and does a KV get and put per rule, even in the CLI. Either accept one logged spurious `enter` on rename (and drop AC12), or justify the cost explicitly.'
severity: significant
resolution: SeenHooks is dropped. A rename posts one logged enter for the new id, consistent with D10. AC12 is removed.
status: addressed
---
