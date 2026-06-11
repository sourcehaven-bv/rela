---
id: RR-VF69XT
type: review-response
title: errors.test.ts left normalizeApiError branches unasserted
finding: The contract test skipped the AbortError/CanceledError name-based cancellation branches, correlation_id extraction from a ProblemDetail (http kind), and the problem.status ?? response.status fallback.
severity: minor
resolution: Added it.each rows for both cancellation names, a ProblemDetail correlation_id assertion, and a missing-status fallback case (987 tests pass).
status: addressed
---
