---
id: RR-QCLPQ
type: review-response
title: 'AC #13 stub-repo failure injection has no concrete plan'
finding: |-
    Plan: 'Either a thin mock around repo.Store or a counter-based failure-injecting wrapper.' 'Either' isn't a plan. Repo type's interface and how WithTx interacts with it determines whether you can inject failure at the right point. If repo is concrete (not behind interface), injection requires interface extraction (scope creep) or a hook in tx layer (changes load-bearing code). 1-line bullet hiding potentially half a day of work.

    Fix: confirm injection seam before implementation. Research item: identify lowest-disruption seam to inject write failure on Nth tx.WriteRelation/tx.WriteEntity call. Candidates: (a) repository.Tx already an interface — wrap it; (b) underlying filesystem is tx.repo.fs.Rename — substitute counting filesystem. Pick (b) if available; tests actual commit path. Document chosen approach in plan's Test Plan section.
severity: minor
resolution: 'Test infrastructure section names the failure-injection seam: FS interface (tx.repo.fs.Rename) preferred since it tests actual commit path; falls back to repository.Tx (already an interface). Both verified as feasible during research.'
status: addressed
---
