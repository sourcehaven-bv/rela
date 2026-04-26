---
id: RR-M4EB9
type: review-response
title: 'F1: gopher-lua compile errors have a different shape'
finding: 'The plan''s regex assumption ([string "path"]:line: and path:line:) covers runtime errors but not compile errors. Compile errors look like ''actions/bad.lua line:1(column:7) near \''is\'': parse error'' (line:N(column:N) form). They''re also wrapped by ''cannot compile script: %w'' in runtime.go:360 and 387, so a string regex won''t match either planned pattern. Critical for document/MCP surfaces where compile errors are routine.'
severity: critical
resolution: 'Plan switched from regex-on-err.Error() to typed errors.As(err, **lua.ApiError) extraction. Compile errors (Type==ApiErrorSyntax, wrapped via fmt.Errorf ''cannot compile script: %w'') handled via errors.Unwrap + line:N(column:N) parser. Fallback to message-only on no-match.'
status: addressed
---
