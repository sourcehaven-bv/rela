---
id: RR-H0VKN4
type: review-response
title: Silent baseline runs outside the round lock and is not crash-ordered
finding: §3 Round step 2 scans and writes the shards of a new rule before step 3 takes the lock, so two nodes (or desktop and server on fs) can baseline at the same time and overwrite each other's shards. A full-type Scan inside a 500ms tick also stalls every other rule. Run the baseline under the lock (or as a reconcile-style candidate pass), write the index last, and add a two-engine baseline case to AC10.
severity: significant
resolution: All rounds run in the single leader worker (D1). The baseline is a paged candidate pass inside that worker and writes the index last. A leader takeover test covers the two-engine case.
status: addressed
---
