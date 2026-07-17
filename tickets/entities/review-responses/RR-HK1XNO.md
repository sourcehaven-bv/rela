---
id: RR-HK1XNO
type: review-response
title: 'ACL inference channel: relation filter narrows rows by ungated neighbor titles'
finding: 'resolveRelationColumnValues → entityTitle calls svc.Store.GetEntity directly, bypassing the read gate. Rows are ACL-scoped but the matched neighbor is not, so `filter[verantwoordelijk_voor]=Secret Person` narrows the visible list to exactly the rows edged to a person the caller may not read — leaking that person''s existence and title-match via row inclusion, even on a list that renders no such column. Same root cause as the ODHV2D critical (ungated neighbor reads) and the pre-existing sections.go:257 column path. Make a conscious, documented decision: route neighbor resolution through visibleReader/PermitsRead, or explicitly document relation-target titles as project-world-readable. No ACL test covers this path (nop gate). helpers.go:697,706.'
severity: significant
resolution: matchRelationFilter resolves neighbor titles then gates the title-matching candidates through readGate.PermitsReadMany (batched per neighbor type), so a filter value matching a hidden neighbor does not include the edged rows. Short-circuits on first readable match. Documented to converge on ODHV2D's visibleRelationIDs when merged. Pinned by TestV1ListRelationFilterACL (hidden neighbor → 0 rows; visible neighbor still matches). Commit 72f10b99.
status: addressed
---
