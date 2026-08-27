---
id: RR-HVJOWP
type: review-response
title: 'Third upstream data race: neoq Shutdown vs Start on cancelFuncs'
finding: neoq's memory backend reads and nils m.cancelFuncs in Shutdown WITHOUT holding m.mu, while Start appends to it under that mutex — a clean data race under -race. neoqQueue released its own lock before calling into neoq, so nothing serialized the two. rela runs -race across CI and forbids //go:build !race, so this was a third upstream race beyond the two already accepted.
severity: significant
resolution: neoqQueue.Start and Close now hold q.mu ACROSS the call into neoq (defer Unlock rather than unlocking first), which is the only containment available from outside the library. Comments at both sites name the upstream cause so the lock scope is not 'tidied' back.
status: addressed
---
