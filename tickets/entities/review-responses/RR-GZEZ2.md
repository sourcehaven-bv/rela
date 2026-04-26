---
id: RR-GZEZ2
type: review-response
title: formatScriptError mishandles multi-line LuaMessage
finding: 'When se.Error() contains a newline (wrapped context.DeadlineExceeded with frame, or any Lua error message containing newline), the headline writes through unchanged. The 240-char cap doesn''t address embedded newlines, so rendered output can have a multi-line ''headline'' before the source slice. Operator UX gets ragged. Location: internal/cli/scripterror_format.go:35-54.'
severity: minor
resolution: formatScriptError now passes the headline through collapseHeadline before length-capping. Newlines (CR/LF/CRLF) are replaced with spaces and runs of whitespace are collapsed via strings.Fields, so wrapped DeadlineExceeded or multi-line Lua errors render as one normalised headline above the source slice. Two new tests cover the collapse and collapse+truncate paths. Commit 0711a6b.
status: addressed
---
