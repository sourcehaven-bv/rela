---
id: RR-XMJX1
type: review-response
title: TestLoad_AllShippedMetamodels silently ignores catalog-metamodel.yaml
finding: 'loader_test.go hard-codes filename match info.Name() != "metamodel.yaml" and silently skips anything else. The repo ships prototypes/data-entry/catalog-metamodel.yaml — a real loadable metamodel — which is silently excluded. Confirmed: the test currently runs against 5 files, not 6. Per RR-G175B the intent was "guard against typos in dogfood and fixture metamodels." Either widen the match (e.g. *metamodel*.yaml) or add a sentinel assertion that the count meets a known floor / a known file is present. Either way, rename the test or update its docstring so the actual coverage matches the claim.'
severity: significant
resolution: Switched from filename equality to suffix match (*metamodel*.yaml). TestLoad_AllShippedMetamodels now finds 6 metamodels (was 5) including catalog-metamodel.yaml. Added .ignored, fixtures, testdata to skip set. Test name and docstring rewritten to be honest about coverage.
status: addressed
---
