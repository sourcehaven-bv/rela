---
id: RR-SZ2EJKZ
type: review-response
title: A needle that shares no trigram with any title still sorts every match
finding: 'Search orders by distance then id. Rows whose title shares no trigram with the needle all have distance 1.0 so the incremental sort must read that whole tie group: nearly every match when the word is in bodies but in fewer titles than the limit. The same holds right after the upgrade while rows still hold the empty default.'
severity: significant
reason: 'Measured on the scale-5 seed: body-only common words such as precondition (88k matches and no title containing it) still share trigrams with many titles and run in 40 ms reading 27 rows. Only a needle with no trigram in common with any title or the minutes before the first heal pass after the upgrade hit the tie group and those cost what every search cost before this change. Splitting the query at the tie needs a bounded index condition that GiST KNN cannot express without changing pg_trgm settings per query; tracked in TKT-2EST91Z.'
status: deferred
---
