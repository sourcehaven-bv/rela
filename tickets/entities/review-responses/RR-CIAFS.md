---
id: RR-CIAFS
type: review-response
title: 'F11: inner error / Unwrap is speculative with no caller'
finding: Plan adds inner error field and Unwrap() method. Nothing in the codebase walks Lua errors via errors.As for *lua.ApiError. Either drop the field or note it's speculative.
severity: nit
resolution: Dropped inner field and Unwrap(); plan struct comment notes 'no current caller walks via errors.As'.
status: addressed
---
