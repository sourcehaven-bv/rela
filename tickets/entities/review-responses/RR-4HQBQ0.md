---
id: RR-4HQBQ0
type: review-response
title: Commit reload targets the scope navigated away from
finding: refresh called fetchScope(fetchedRoot), which still names the old root while a drill fetch is in flight; the reload superseded the drill and showed the old scope under the new breadcrumb.
severity: significant
resolution: fetchScope records requestedRoot at start; refresh reloads requestedRoot. View test holds the drill fetch open across the write; it fails with the old code.
status: addressed
---
