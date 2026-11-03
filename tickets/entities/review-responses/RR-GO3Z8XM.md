---
id: RR-GO3Z8XM
type: review-response
title: archguard fails on the new bare Ref and ParseRef in analyze.go
finding: 'Code review: TestNoNewBareRefs and TestNoNewParseRefAtEdges fail on analyze.go: a bare entity.Ref for the orphan re-load and entity.ParseRef on a violation id.'
severity: critical
resolution: Violation ids are parsed with entity.ParseAddress; an unnamed address becomes a world ref. analyze.go has a bareref allowlist entry (2) saying both are world addresses, as GetAddress read them. go test ./internal/archguard passes.
status: addressed
---
