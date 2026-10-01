---
id: RR-VO2DRJ
type: review-response
title: Usage counter hides an unset families scope
finding: NewStoreCounter did not check its families scope and dropped count errors, so a wiring mistake listed every type as unused.
severity: significant
resolution: NewStoreCounter returns an error for a nil store or an unset scope; StoreCounter.Err surfaces the first failed count and the CLI and MCP callers check it.
status: addressed
---
