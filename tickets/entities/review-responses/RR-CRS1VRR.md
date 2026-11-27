---
id: RR-CRS1VRR
type: review-response
title: Gate errors still turn an id into a title
finding: '[security] ResolveIDsErr logs a per-type gate error and drops that type''s ids without returning an error. A readable target then becomes a title and filter[rel][ne]=id admits every row. The doc comment promises a fail-closed result it only gives for header-read failures. Fix: resolve with a strict callback that returns gate errors as errListLoad.'
severity: minor
resolution: The lookup uses the new visibility.Resolver.ResolveIDsStrict, which returns gate errors; the list fails with errListLoad. Covered by TestRelationFilter_LookupFaultFailsTheList.
status: addressed
---
