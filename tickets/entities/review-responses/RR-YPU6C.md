---
id: RR-YPU6C
type: review-response
title: 'Open-redirect: return_to accepts //evil.com and /\evil.com'
finding: 'internal/dataentry/api_v1.go:2040-2043 checks strings.HasPrefix(returnPath, "/") — passes for //evil.com (protocol-relative URL; browsers resolve off-origin) and /\evil.com (browsers normalise \ to /). Same weak guard at frontend/src/api/documents.ts:18 and frontend/src/components/forms/DynamicForm.vue:117/363/375. Verified: Go run shows all three bypass strings pass HasPrefix. Fix: extract isSafeReturnPath helper that uses url.Parse and asserts Scheme+Host empty plus a leading /. Four call sites (1 Go + 3 TS). Add regression tests for //x, /\x, /%2F%2Fx. Classed critical because I introduced this while claiming to guard against open redirect.'
severity: critical
resolution: Extracted isSafeReturnPath in both Go (internal/dataentry/return_path.go) and TS (frontend/src/utils/returnPath.ts). Go uses url.Parse + Scheme/Host empty check plus explicit rejection of //, /\, /%5C, /%2F prefixes. TS mirrors the shape with new URL() against a placeholder origin and the same prefix rejections. 15 Go test cases + 22 TS test cases covering every known bypass (protocol-relative, backslash, percent-encoded separators, http/https/mailto/javascript/data schemes, empty/non-slash inputs). Wired into api_v1.go:2040, api/documents.ts, DynamicForm.vue (via readReturnTo util). Manually verified with curl //evil.com → rejected, zero occurrences in rewritten HTML.
status: addressed
---
