---
id: PLAN-54LN
type: planning-checklist
title: 'Planning: Add shebang support to Lua scripts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN SCOPE:
- Strip shebang lines (`#!...`) from Lua scripts before execution
- Support shebang in scripts loaded from `scripts/` directory (CLI, MCP lua_run)
- Support shebang in validation scripts loaded from `validations/` directory
- Support shebang in automation Lua file actions

OUT OF SCOPE:
- Actually making scripts executable via shebang (that's a shell/OS concern)
- Modifying the `rela script` CLI to accept scripts from stdin
- Adding a `rela-lua` binary/symlink for direct script execution

**Acceptance Criteria:**
1. Scripts starting with `#!/...` execute successfully (shebang line stripped)
2. Scripts starting with `#!` on first line only (not in middle of file) are handled
3. Scripts without shebang continue to work unchanged
4. Shebang stripping works for all entry points: `rela script`, MCP `lua_run`, automation engine, validation engine
5. Error line numbers in Lua errors remain accurate (shebang line should not shift line numbers)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**
- Standard Lua interpreter handles shebangs natively by ignoring lines starting with `#`
- gopher-lua (the library used) does NOT handle shebangs - it will fail on `#!`
- The fix is simple: strip the first line if it starts with `#!` before passing to the Lua VM
- No existing pattern in codebase for this - this is new functionality

**Entry points that load Lua scripts:**
1. `internal/lua/runtime.go:147-162` - `RunFile()` uses `L.DoFile(path)` directly
2. `internal/script/executor.go:81-119` - `loadScript()` reads file, returns string
3. `internal/mcp/tools_lua.go:118-141` - `handleLuaRun()` reads file, calls `RunString()`
4. `internal/validation/lua.go:207-245` - `loadScript()` reads file, returns string

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add a `StripShebang(code string) string` function in `internal/lua/runtime.go`
that:
1. Checks if the string starts with `#!`
2. If so, finds the first newline position
3. Returns the substring starting FROM (including) the newline - this preserves line numbers
4. If no newline found (single-line shebang), returns empty string
5. If doesn't start with `#!`, returns the string unchanged

Example: `#!/bin/rela\ncode` becomes `\ncode` where line 1 is blank and line 2
is `code`.

Call this function in `RunString()` - this covers all entry points since MCP,
validation, and automation all funnel through `RunString()`.

For `RunFile()`: read the file, strip shebang, use `L.LoadString()` with chunk
name set to the filename (preserves filename in error messages), then call the
loaded function.

**Alternatives considered:**
1. **Strip in each loader separately** - Rejected: duplicates logic in 4 places
2. **Modify gopher-lua** - Rejected: external dependency, unnecessary
3. **Strip in RunString only** - Chosen: single point of change, all paths go through RunString

**Dependencies:**
- No new packages needed
- Only modifies `internal/lua/runtime.go`

**Files to modify:**
- `internal/lua/runtime.go` - Add `StripShebang()` function, modify `RunString()` to call it, modify `RunFile()` to read file, strip shebang, use `LoadString()` with chunk name
- `internal/lua/runtime_test.go` - Add tests for shebang handling

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Input: Lua script content (string) from files in `scripts/` or `validations/` directories
- Validation: Only strip `#!` from the very first characters of the string - this is safe
- Invalid input: If script doesn't start with `#!`, return unchanged - no error

**Security-Sensitive Operations:**
- None added - this is a simple string manipulation before existing secure execution
- File access is already secured via `os.OpenRoot` in the loaders

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: Test script with `#!/usr/bin/env rela script` shebang executes correctly
2. AC2: Test script with shebang-like content in middle of file (not stripped)
3. AC3: Test script without shebang continues to work
4. AC4: Test `RunFile()` with shebang script executes correctly
5. AC5: Test error in line 2 of shebang script reports as line 2 (not line 1)
6. Test `RunFile()` error messages include filename (not `<string>`)

**Edge Cases:**
- Empty script (just shebang, no code) → returns empty string, executes as no-op
- Shebang with no newline → returns empty string
- Script starting with `#` but not `#!` (e.g., Lua comment) → not stripped (Lua comments use `--`)
- Windows line endings (`\r\n`) → find `\n`, let `\r` be part of blank line (harmless)
- Very long shebang line → handled correctly

**Negative Tests:**
- Script with syntax error after shebang → error with correct line number
- No negative tests for shebang itself - invalid shebangs just get stripped

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
1. **Line number accuracy in errors** - Risk: stripping line shifts line numbers
   - Mitigation: Return substring starting FROM the newline (include it) so line count is preserved

2. **Performance** - Risk: checking every script
   - Mitigation: Single string prefix check, negligible overhead

**Effort: xs** (extra small) - Simple string manipulation, ~20 lines of code +
tests

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed
- Scripts with shebangs will "just work" - no documentation required
- Could optionally add a note to CLI help for `rela script` but not required

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-X4B5, RR-UWZW, RR-V824 (all minor, addressed in
updated plan)
