---
id: RR-0GUJ
type: review-response
title: os.OpenRoot requires Go 1.24+
finding: os.OpenRoot was added in Go 1.24. If the project compiles on older Go versions, this will be a runtime panic or compilation error. There is no build tag or Go version constraint. The go.mod should require Go 1.24+.
severity: critical
resolution: go.mod already specifies go 1.24 and toolchain go1.24.13, which ensures os.OpenRoot is available.
status: addressed
---
