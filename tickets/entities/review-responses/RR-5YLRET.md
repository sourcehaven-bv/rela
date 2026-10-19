---
id: RR-5YLRET
type: review-response
title: '`failures` map entries leak when an effect disappears before it succeeds or exhausts its retries'
finding: 'internal/realtime/trigger.go:907-916. failures[k] is deleted only on success, skip or give-up. If the entity changes so the effect no longer arises (it stops matching, is deleted, or the rule is edited or removed and gets a new id), the key stays forever. A later failure for the same rule+ref also starts from the stale count and gives up early. Fix: clear failures for ids processed in the round that produced no effect, or store the count with a timestamp and prune it with lastPost at the top of RunRound.'
severity: minor
resolution: Failure entries are pruned at the start of each round once they are older than twice the retry window, and Reset clears them.
status: addressed
---
