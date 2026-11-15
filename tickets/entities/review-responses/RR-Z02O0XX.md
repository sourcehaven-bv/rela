---
id: RR-Z02O0XX
type: review-response
title: Test fake duplication and doc nits
finding: firstEqual fake duplicated in ingest and subscription tests; store doc overstated the mem/fs cost; fmt.Sprint vs pg number text.
severity: nit
resolution: Store doc now says memstore and fsstore scan their in-memory index; the number text difference is recorded in the planning risks. The fakes live in different packages and stay separate.
status: addressed
---
