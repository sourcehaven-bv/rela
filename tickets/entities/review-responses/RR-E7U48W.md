---
id: RR-E7U48W
type: review-response
title: Error prefixes changed
finding: 'GraphQueryHeaders errors changed from ''pgstore: graph query headers'' to ''pgstore: graph query''.'
severity: minor
resolution: graphPages takes the prefix; both methods keep their original prefixes. Entity and relation listing errors were unwrapped before and still are.
status: addressed
---
