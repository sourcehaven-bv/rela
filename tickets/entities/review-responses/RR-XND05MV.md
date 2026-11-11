---
id: RR-XND05MV
type: review-response
title: MCP post with files takes no upload slot
finding: post_message with files bypassed the shared upload limiter that attach_file uses.
severity: significant
resolution: handlePost takes a slot from Deps.Attachments.Uploads when files are present. Tested in mcp TestChannelFiles_BusyUploads.
status: addressed
---
