---
id: RR-M6JPEW
type: review-response
title: walkGraphQueryPages guards page-count but not cursor advance or duplicate ids
finding: internal/store/storetest/graphquery.go:570-580. The walker requires NotEmpty(page.Items) but never checks the cursor actually advanced. A backend re-serving the same non-empty page with the same cursor burns all 100 iterations and reports 'did not terminate within 100 pages' — a failure, but the diagnostic names the bound instead of the real defect, after accumulating 100 pages of duplicate ids. Also nothing asserts ids are strictly increasing across the page boundary; Page_stable_order_across_calls checks slices.IsSorted on the concatenation but sorted-but-duplicated passes IsSorted, and that is one subtest rather than the shared walker.
severity: minor
resolution: 'Both guards added to walkGraphQueryPages so every paged subtest inherits them. (1) Cursor advance: require.NotEqual(cursor, page.NextCursor) before reassigning, so a backend re-serving the same page fails on iteration 2 naming the real defect instead of burning 100 iterations and blaming the bound. (2) Strict increase: each id is asserted greater than the previous across the whole walk, including page boundaries — which catches duplicates that slices.IsSorted cannot, since sorted-with-duplicates is still sorted. Kept maxPages=100 as the backstop for any case the two new guards miss.'
status: addressed
---
