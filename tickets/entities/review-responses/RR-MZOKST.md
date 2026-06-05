---
id: RR-MZOKST
type: review-response
title: Only the 3 entity broadcasts are safe to remove; git/refresh broadcasts must stay
finding: 'Design-review verification: the full set of dataentry broadcast call sites is: api_v1.go:592 (entity created), :905 (entity updated, gated on entityChanged), :963 (entity deleted) — these 3 go through entityManager->store, so the store-event bridge covers them and they''re SAFE TO REMOVE. BUT three others must NOT be touched: handlers_git.go:104 broadcastGitStatus (after git merge/rebase, no store write), watcher.go:115 broadcast(''refresh'') (config data-entry.yaml change), watcher.go:164 broadcast(''git'') (after git fetch). The plan said ''remove the 3 inline broadcasts'' which is correct, but the implementer must NOT over-remove — only the 3 entity ones.'
severity: minor
status: open
---

## Resolution (plan update)

Pin the exact removal list in the plan: remove ONLY
`broadcastEntityEvent("created"/"updated"/"deleted", ...)` at api_v1.go:592,
905,
963. Leave untouched: handlers_git.go:104 (git status), watcher.go:115
(config refresh), watcher.go:164 (git fetch) — none involve a store write, so
the store-event bridge does not cover them and they remain the broadcast source
for their respective triggers.
