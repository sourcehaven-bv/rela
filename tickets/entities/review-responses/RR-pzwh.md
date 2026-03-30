---
finding: query.go breaks on EOF or error silently, treating all errors as EOF. If rowIter.Next() returns an actual error, partial results are returned with no indication of failure.
id: RR-pzwh
resolution: Changed from 'break // EOF or error' to properly check 'errors.Is(err, io.EOF)' and return error for non-EOF errors
severity: critical
status: addressed
title: Silent error swallowing in row iteration
type: review-response
---
