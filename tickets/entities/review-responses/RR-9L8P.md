---
id: RR-9L8P
type: review-response
title: 'F9: aiErrorToTable drops cause, making Lua-side debugging painful'
finding: The Lua error table exposed kind, status, message, retry_after. The wrapped cause error chain was discarded. For rare/flaky failures where the Go-level error wrapping includes helpful transport detail (TLS cert issue, DNS record, etc.), the script author only saw the top-level Message.
severity: minor
resolution: Added a 'details' field to the Lua error table via errors.Unwrap(e). Empty when there is no cause; otherwise contains the unwrapped error string. Documented the field in aiErrorToTable's doc comment so future maintainers understand its purpose.
status: addressed
---
