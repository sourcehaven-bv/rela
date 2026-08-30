---
id: RR-CNY4SZ
type: review-response
title: Mangled duplicate doc comment on stateKVFor
finding: versionsweep_postgres.go carried a truncated fragment of the stateKVFor doc comment (ending mid-sentence at 'a second backend gets version sweeps, user state and derived') immediately followed by a restart of the whole block — the residue of a bad edit splice. A reader hits a sentence that stops mid-clause and then reads the same explanation again.
severity: significant
resolution: Deleted the orphaned fragment, keeping the single intact block.
status: addressed
---
