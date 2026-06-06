---
id: RR-REQW
type: review-response
title: TKT-VMD8 AC6 sidebar config-filter perf cliff is unflagged
finding: 'Intersect-then-filter approach is correct but plan doesn''t quantify when it falls over. For a principal who sees 100k entities and a config filter matching 50, iterates 100k Go values per sidebar paint. 20-item sidebar = 2M iterations per page load. Add a note in AC6: ''Sidebar config filters under ACL evaluate in-memory after the ACL GraphQuery. Performance scales with visible-set size, not total entities. For visible sets >10k, prefer pre-filtering via entity_type in navigation config or filing a follow-up to push filter predicates into GraphQuery.'' Don''t fix in this PR — pin the gap so the next person isn''t surprised.'
severity: significant
status: open
---
