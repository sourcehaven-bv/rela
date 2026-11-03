---
id: RR-80XH8N
type: review-response
title: Conformance subtests never tested fs incremental maintenance
finding: fsstore builds lazily, so subtests that query only after writing exercised only the build, not the write hooks.
severity: minor
resolution: RunMentionTests runs every case Cold and Warm; Warm queries first so writes must update a built index. Removing the writeEntity hook now fails five Warm cases on fs.
status: addressed
---
