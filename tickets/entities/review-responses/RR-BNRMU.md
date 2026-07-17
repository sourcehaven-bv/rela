---
id: RR-BNRMU
type: review-response
title: Date-literal parse layouts narrower than filter's ParseDateValue
finding: 'dateparse.go defaultDateLayouts = {2006-01-02, RFC3339} (2 layouts). filter''s metamodel.ParseDateValue falls back through 4 (RFC3339, ...Z, ...T15:04:05 no zone, date-only). A literal filter''s --where accepts (e.g. 2026-02-01T10:00:00 no zone) fails predicate coercion when the field layout doesn''t cover it. For a ''typed superset of filter'' this is a parity regression. Fix: align the fallback list with metamodel''s, or document the intentional narrowing.'
severity: minor
resolution: 'dateparse.go defaultDateLayouts now mirrors metamodel.ParseDateValue''s fallback set exactly: RFC3339, ''2006-01-02T15:04:05Z'', ''2006-01-02T15:04:05'' (no zone), ''2006-01-02'' (date-only). A date literal that filter''s --where accepts now also coerces in predicate, restoring superset parity.'
status: addressed
---
