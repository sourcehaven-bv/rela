---
id: RR-X5A478J
type: review-response
title: The rewritten shell drops the file server headers
finding: For a shell path the new handler writes the bytes itself, so Last-Modified and range support from http.FileServer are gone for the setting-up page.
severity: minor
resolution: 'Kept: this matches spaHandlerWithCustom, which serves a tenant''s rewritten shell the same way. The shell is small and Content-Type and Content-Length are set. TestBootShellHandler pins GET, HEAD and that assets still go to the file server.'
status: addressed
---
