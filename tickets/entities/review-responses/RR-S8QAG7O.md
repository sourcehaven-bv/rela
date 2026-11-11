---
id: RR-S8QAG7O
type: review-response
title: Headless test strategy undefined for SPA-dependent tests
finding: Running the whole internal/dataentry or cmd/rela-server suite with -tags headless would hit existing tests that expect /static/ and the SPA shell. The plan says CI tests the tag but not which tests, so the job either fails or gets weakened ad hoc.
severity: significant
resolution: CI headless job runs go vet -tags headless on all packages (compiles every test file) and go test -tags headless -run Headless on internal/dataentry and cmd/rela-server. Headless tests live in headless_test.go behind the build tag.
status: addressed
---
