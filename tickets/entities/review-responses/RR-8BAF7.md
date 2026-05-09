---
id: RR-8BAF7
type: review-response
title: PATCH detection has TOCTOU race with concurrent git-crypt unlock
finding: 'Plan: PATCH re-reads file, magic-header-checks, returns 422 if encrypted. But between check and write, the user may run git-crypt unlock — the write proceeds against newly-decrypted file but uses the SPA''s stale form data, overwriting the just-decrypted real content. Conversely, if file became encrypted between SPA load and PATCH, write overwrites ciphertext with cleartext, destroying encrypted content. Encrypted-state detection alone does not protect against this. Add explicit AC: PATCH must check optimistic concurrency (mtime/etag) in addition to encrypted-state, and refuse if mtime changed since SPA loaded. Test: simulated race in handlers_api_test.go.'
severity: significant
resolution: Plan now has explicit AC7 (save-path safety) and AC8 (re-read on PATCH). Server preserves on-disk values for any field in on_disk.Inaccessible regardless of what client posts. PATCH re-reads + re-detects before write; if file is now inaccessible, reject. mtime/etag optimistic concurrency is explicitly out of scope (separate ticket); the encrypted-state-check-at-write covers the git-crypt-specific race.
status: addressed
---
