---
id: RR-M8GI9
type: review-response
title: G120 (MaxBytesReader on form parsing) is cheap to fix now -- don't defer to FEAT-ESLP
finding: 'In internal/dataentry/handlers.go:38, handleToggleCheckbox calls r.ParseForm() without wrapping r.Body with http.MaxBytesReader. The gosec G120 warning was excluded in .golangci.yml along with the other taint-analysis G7xx checks and deferred to FEAT-ESLP. But G702-G706 are architectural concerns that warrant a threat-model discussion; G120 is a two-line DoS mitigation that needs no design work: r.Body = http.MaxBytesReader(w, r.Body, 1<<20). Deferring this specific one keeps a known memory-exhaustion hole open while FEAT-ESLP waits. Fix now; keep the G7xx defer as-is.'
severity: significant
resolution: Added r.Body = http.MaxBytesReader(w, r.Body, maxFormBody) (1 MiB) to handleToggleCheckbox in internal/dataentry/handlers.go. Extracted maxFormBody const for future form handlers. Removed G120 from the .golangci.yml gosec excludes so future form parsers must either wrap the body or explicitly suppress. Kept G702-G706 excluded (FEAT-ESLP scope).
status: addressed
---
