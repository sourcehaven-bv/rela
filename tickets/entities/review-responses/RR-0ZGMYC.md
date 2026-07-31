---
id: RR-0ZGMYC
type: review-response
title: tree check passes on a zero-byte entry asset
finding: 'The assets check uses `find -type f -name ''index-*''` with no size predicate, while index.html correctly uses `-s`. Verified false PASS: a tree with index.html plus a zero-byte assets/index-AAAA.js returns ''OK: SPA build tree populated'', rc=0. A truncated or interrupted Vite write is exactly the ''silently produced nothing'' case the comment claims to catch. Add a non-empty predicate (-size +0 / ! -empty). The binary check would still catch this, but the tree check exists to fail early with a clear message.'
severity: significant
resolution: 'The tree check no longer globs for a name pattern. It parses the asset paths index.html actually references and requires each to exist AND be non-empty (`[ ! -s ]`). Pinned by two tests: ''zero-byte entry asset fails'' and ''referenced asset absent from disk fails'', both rc=1. Also added an explicit `-d assets/` check so a missing assets dir reports its own diagnosis rather than the misleading ''no bundle'' message (the RR-88CBD0(c) point).'
status: addressed
---
