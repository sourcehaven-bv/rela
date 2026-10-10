---
id: RR-J63GFG
type: review-response
title: maildemo/mailmanual and OS-specific files still unlinted
finding: Those tags add test files that no run sees; windows files are also unseen.
severity: minor
resolution: 'The default step now passes --build-tags maildemo,mailmanual (fixed one stale nolint it surfaced). Windows files stay out of scope: they need a GOOS=windows run with the Wails cgo deps.'
status: addressed
---
