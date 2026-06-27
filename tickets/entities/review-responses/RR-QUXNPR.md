---
id: RR-QUXNPR
type: review-response
title: Dates/timestamps (time.Time) hit default branch and diverge
finding: 'REPRODUCED. yaml.v3 decodes `due: 2026-06-19` to time.Time, not string. canonicalValue has no time.Time case → falls to default (canonical.go:177-183) → fmt.Sprintf ''%v'' = ''2026-06-19 00:00:00 +0000 UTC''. The pg path never sees time.Time: JSONB stores it as RFC3339 string and reads back a string → canonical emits s:2026-06-19T00:00:00Z. So u:2026-06-19 00:00:00 +0000 UTC (fs) vs s:2026-06-19T00:00:00Z (pg) → divergent hash. Dates are extremely common in this project''s frontmatter (due dates, decision dates, accepted dates — the metamodel uses date types). Breaks sync for ANY dated entity. The default/u: branch is the red flag; time.Time is the value type that hits it in real data. FIX: handle time.Time explicitly with a fixed format, or normalize at the store boundary (L2).'
severity: critical
resolution: normalize() folds time.Time -> val.UTC().Format(RFC3339Nano), emitted with the string kind. This matches pgstore's round-trip (time.Time -> RFC3339 string on read) and correctly makes a date and a user-typed identical string hash alike (pg cannot distinguish them). Regression TestHashEntity_DateEqualsString and the date/datetime cases in TestHashEntity_CrossBackendDecode assert it. The silent default branch that previously caught time.Time is gone.
status: addressed
---
