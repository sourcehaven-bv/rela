---
id: RR-GKZELE
type: review-response
title: TAB after //go:build made the guard silently blind
finding: 'scripts/check-tagged-tests.sh: discovery matched a literal space after the directive (/^\/\/go:build /p), but Go splits build constraints on any whitespace. A file constrained with //go:build<TAB>alpha is therefore live and excluded from the default build, yet the guard reported "No build-tag-gated files found; nothing to compile" and exited 0. Verified against the toolchain: untagged `go vet ./...` exits 0 (file excluded) while `go vet -tags alpha ./...` reports the broken call. This is the exact silent-pass that let e2e_test.go rot for months, reproduced inside the tool built to prevent it — worse than no guard, because a green check now stands between the rot and anyone looking.'
severity: critical
resolution: Root-caused as hand-rolling a parser that ships with Go. Replaced sed/grep discovery with scripts/tagged_build_tags.go, which uses go/build/constraint.Parse and is correct by construction for any whitespace. Regression cases "TAB after //go:build parsed" and "rotted TAB-constrained file FAILS" (the latter using a real tab via printf) assert both the discovery and the failure.
status: addressed
---
