---
id: RR-MWCAU
type: review-response
title: 'F6: 500 vs 422 split - distinguish Lua errors from action-contract errors'
finding: 'actions_test.go:214 (TestHandleV1Action_OpenRedirectRejected) asserts HTTP 500 on a script that returns an invalid redirect. That''s a contract failure from validateRedirect/parseActionResponse, not a Lua failure. After this change, only Lua execution errors should become 422+envelope; contract errors stay on the 500+action_failed path. Plan should explicitly call out: dataentry handler does errors.As(err, **lua.ScriptError); script.Engine.ExecuteAction wraps the Lua error itself so contract errors aren''t wrapped.'
severity: significant
resolution: 'AC #1 explicitly says non-Lua errors keep 500+action_failed shape. script.Engine.ExecuteAction wraps Lua errors in *lua.ScriptError itself; contract errors from parseActionResponse/validateRedirect are NOT wrapped. Handler does errors.As to branch. Existing TestHandleV1Action_OpenRedirectRejected continues to assert 500.'
status: addressed
---
