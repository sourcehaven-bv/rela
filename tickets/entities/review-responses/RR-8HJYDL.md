---
id: RR-8HJYDL
type: review-response
title: docscapture NewApp call site missed in the plan's file list
finding: The plan lists rela-server and rela-desktop as NewApp call sites to update, but internal/docscapture/server.go:97 ALSO calls dataentry.NewApp and was not mentioned. It's a headless internal capture server; if it's left passing no authorizer (or the param is added without updating it) the build breaks, or worse, if a nil default is silently accepted it fails open. Any positional-param change to NewApp must update all THREE production sites.
severity: significant
resolution: Plan file list corrected to include internal/docscapture/server.go:97. docscapture is a local doc-render harness with no network bind and no untrusted clients, so it passes ungatedAuthorizer{} (loopback-equivalent), consistent with desktop. A nil authorizer at the NewApp boundary is rejected (treated as deny) so a forgotten site fails closed and loudly, never open.
status: addressed
---
