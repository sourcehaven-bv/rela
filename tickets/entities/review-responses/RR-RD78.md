---
id: RR-RD78
type: review-response
title: FS-layer decorator alternative dismissed too quickly
finding: 'Plan rejects FS-layer (storage.RootedFS) detection with ''would intercept metamodel/templates too, with no benefit.'' But intercepting metamodel/templates IS a benefit — if metamodel.yaml is encrypted, today rela explodes on startup with binary YAML parse error; an FS-layer decorator could surface ''your metamodel is encrypted, run git-crypt unlock'' clearly. Reconsider: detection at storage.RootedFS.ReadFile is a single insertion point that covers fsstore + metamodel loader + watcher (closes the watcher gap from finding #2 for free). If staying at fsstore, document the rejection as ''out of scope, not no-benefit'', and explicitly accept that encrypted metamodel/templates will fail with the existing error path.'
severity: minor
reason: FS-layer detection considered but rejected for this ticket. Under partial encryption (the norm with git-crypt), metamodel.yaml is normally cleartext, so the metamodel-coverage benefit is small. fsstore-internal detection at readDataFile covers all entity/relation read paths via a single insertion and avoids touching the storage interface. Plan now documents the rejection as 'out of scope' rather than 'no benefit.' Revisit if a future ticket targets encrypted metamodel.
status: wont-fix
---
