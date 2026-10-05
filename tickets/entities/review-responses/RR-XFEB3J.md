---
id: RR-XFEB3J
type: review-response
title: Ref lives in face.go but is tested in ref_test.go
finding: The Ref type was appended to face.go while its tests live in ref_test.go.
severity: minor
resolution: Moved Ref and its methods to internal/entity/ref.go.
status: addressed
---
