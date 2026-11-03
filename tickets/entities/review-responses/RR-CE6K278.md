---
id: RR-CE6K278
type: review-response
title: Contract CI path filter too narrow
finding: The behaviour the fake copies is spread over internal/dataentry, entitymanager, store, metamodel and visibility. Filtering on internal/dataentry alone lets a change elsewhere make the fake wrong without running the check.
severity: significant
resolution: The contract test runs on every PR that touches internal/**, cmd/rela-server/** or the frontend paths, reusing the CI Go build.
status: addressed
---
