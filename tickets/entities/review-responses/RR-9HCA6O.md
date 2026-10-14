---
id: RR-9HCA6O
type: review-response
title: RequestInfo exists only for log lines
finding: A struct carrying path and method only for logging is an odd API; pass a logger or return errors.
severity: minor
resolution: Open takes a *slog.Logger scoped by the caller with path and method; errors are sentinels (ErrPrincipalMismatch and ErrUnresolvedPrincipal) that wrap the cause.
status: addressed
---
