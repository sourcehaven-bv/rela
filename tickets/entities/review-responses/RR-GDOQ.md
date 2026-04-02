---
id: RR-GDOQ
type: review-response
title: Error messages may leak path information
finding: Error messages include full paths which could leak system information in shared environments. The errors 'cannot open project root' and 'scripts directory not found' wrap the underlying error which may contain absolute paths.
severity: significant
resolution: 'Error messages in loadLuaScript() now omit system paths. Changed ''cannot open project root: %w'' to ''cannot access project directory'', and similar sanitization for other error messages. The script name is still included but no absolute paths are leaked.'
status: addressed
---
