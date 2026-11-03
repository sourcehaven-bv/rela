---
id: RR-F5G8ZD
type: review-response
title: Inline errors report LuaPath <string>
finding: RunString compiles under the chunk name <string> while the envelope Path is <inline>, so every inline error set LuaPath and Error() printed <string>:N.
severity: significant
resolution: LuaPath is set only for a frame under lib/. Pinned by TestBuildScriptError_InlineErrorHasNoLuaPath.
status: addressed
---
