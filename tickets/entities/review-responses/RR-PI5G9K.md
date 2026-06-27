---
id: RR-PI5G9K
type: review-response
title: handleV1App collapses unreadable-file to 404 with no server-side log
finding: 'apps_handler.go:66-71 collapses declared-but-unreadable (missing/oversize/perm error) to a generic 404 with no slog.Warn. Not-leaking is right for the response, but an operator who fat-fingers a filename or whose file is deleted at runtime gets a silent 404. FIX: slog.Warn the real err server-side, keep the generic 404 response.'
severity: minor
resolution: handleV1App now slog.Warn's the real load error (app id + file + err) before returning the generic 404, and slog.Error's a CSP-injection/render failure before the 500. The response stays generic (no existence leak); the operator gets a log line.
status: addressed
---
