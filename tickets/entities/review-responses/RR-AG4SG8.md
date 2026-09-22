---
id: RR-AG4SG8
type: review-response
title: Client-supplied exec_id let one run age another run's download tokens
finding: 'commandFileStore.byRun was keyed on execID, which handleCommandExec reads straight from the query string (?exec_id=). Two runs passing the same id share a byRun bucket, so the first to finish stamps an expiry deadline on the other''s still-live tokens. The sibling registry runningCommands records an `owner` for exactly this hazard (RR-YZV7SY) and the new table inherited the shape without the mitigation. Impact ceiling is a premature 404 on a Download button, not a disclosure: release only ever shortens a lifetime, and mint generates a fresh random token regardless of the key. Found independently by both the security and code reviewers.'
severity: significant
resolution: Added newRunKey() — 128 bits of crypto/rand minted server-side in handleCommandExec — and keyed the token table on it. The client's exec_id still addresses /api/command-cancel/, where the existing owner check already fences it, but it no longer names anything in the download table. Renamed the store's parameters from execID to runKey so the distinction is visible at every use.
status: addressed
---
