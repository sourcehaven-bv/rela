---
id: RR-TV6X80O
type: review-response
title: In-memory bleve in untagged recipe breaks postgres no-bleve gate
finding: ci.yml go list -deps -tags postgres forbids bleve; bleveindex_shared.go is tagged (!postgres && !memorybackend) || sqlite.
severity: critical
resolution: NewInMemory uses search.NewLinearSearch (untagged). Guide states test search ranking is LinearSearch's. CI dependency step gains apptest checks.
status: addressed
---
