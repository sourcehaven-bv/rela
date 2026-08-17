---
id: RR-AROZJY
type: review-response
title: 'Empty-string divergence: index rejects email:'''' rows the scan blesses'
finding: 'checkUniqueProperties skips empty values; a naive index treats explicit '''' as a real value, so two email:'''' rows collide under the index but pass the scan — divergent semantics changed silently on first reconcile. Index predicate must exempt empty: WHERE type=''t'' AND properties->>''p'' <> '''' AND IS NOT NULL. Add a scan-vs-index agreement test (empty/absent/list/non-string).'
severity: significant
status: open
---

`checkUniqueProperties` skips empty values (`if v := e.GetString(name); v !=
""`, unique.go:69) and list properties. A naive partial index `ON entities
(type,(properties->>'p')) WHERE type='t'` diverges: `properties->>'absent'` is
SQL NULL → distinct under a unique index (safe, matches scan), BUT an
explicitly-written `''` is NOT NULL → two entities with email:'' collide under
the index yet PASS the scan (GetString returns "" for both missing and
explicit-empty; JSONB ->> distinguishes them). The "friendly scan primary, index
backstop" story breaks — the index rejects a write the scan blessed, 422 with no
friendly message ever fired. On first reconcile this silently changes uniqueness
semantics for empty values across EVERY existing deployment.

REQUIRED: the index predicate must exempt empty string to match scan semantics:
`WHERE type='t' AND properties->>'p' <> '' AND properties->>'p' IS NOT NULL`.
State it explicitly; add an AC/test asserting scan-and-index agreement on empty,
absent, list, and non-string values.
