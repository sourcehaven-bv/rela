---
id: RR-O8TE9TN
type: review-response
title: Lower returned a whole GraphQuery fragment
finding: QueryConditions.Lower returned a store.GraphQuery and the handler copied only Props and Related, so any other field the lowering set would be dropped silently.
severity: significant
resolution: Lower now returns props, related, empty and ok. The fragment never leaves appbuild.
status: addressed
---
