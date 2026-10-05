---
id: RR-2JJ55O
type: review-response
title: Direct-read guard misses unkeyed EntityQuery literals
finding: directReads matches an EntityQuery literal only when it sets the IDs key. A positional literal is not caught and the doc did not say so.
severity: nit
resolution: The directReads doc now lists the unkeyed literal as not caught and notes that go vet already rejects positional literals of an imported struct.
status: addressed
---
