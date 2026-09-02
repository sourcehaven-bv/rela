---
id: RR-UZ83ET
type: review-response
title: statekv.go doc referenced a symbol that does not exist
finding: 'The doc comment in internal/store/pgstore/statekv.go pointed the reader at ''the storeStateProvider interface in appbuild'' to explain where consumer-side widening happens. No such symbol exists — it is named rawStateStore. A dangling cross-package reference is worse than none: it sends a reader grepping for something that was never there, and commentlint''s doclink gate cannot catch it because it is prose, not a [Bracketed.Reference].'
severity: significant
resolution: Corrected to rawStateStore.
status: addressed
---
