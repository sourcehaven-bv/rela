---
id: RR-L0ZGRI
type: review-response
title: Generic impl missing ValidateFilters — filter-rejection asymmetry between implementations
finding: pgstore.SearchVisible validates filters up front; the generic Visible forwarded them to the inner Service, which rejects via a different path. The Filters dimension of the VisibleSearcher contract was effectively unspecified — the conformance suite never asserted filter-validation behavior.
severity: significant
resolution: ValidateFilters hoisted into the generic impl (visible.go visibleHits, before any backend/scope work) for parity; new conformance case InvalidFilterRejectedIdentically asserts BOTH impls reject an ordered filter with the same sentinel (search.ErrOrderedFilterUnsupported) and yield zero hits before the error.
status: addressed
---
