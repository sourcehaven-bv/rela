---
id: RR-W2KL2
type: review-response
title: RunValidationString does not strip shebang
finding: 'Inline lua: blocks won''t have shebangs, but lua_file: rules might (someone copying a script that was previously executable). New method skips stripShebang while every sibling Run* method strips it. Quiet inconsistency that will bite when someone copies a CLI Lua script into validations/. Location: internal/lua/runtime.go:493-515 vs :430-477 (RunActionString).'
severity: minor
resolution: 'Added stripShebang to RunValidationString matching sibling Run* methods. Test TestRunValidationString_StripsShebang verifies a #!/usr/bin/env lua header is stripped before compile. Commit 4f9166a.'
status: addressed
---
