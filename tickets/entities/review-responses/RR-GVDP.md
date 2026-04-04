---
id: RR-GVDP
type: review-response
title: Missing test coverage for path traversal attacks on lua_file
finding: 'The `loadScript()` function at `/Users/jeroen/Work/sourcehaven/rela-3/internal/validation/lua.go:106-144` has good security measures (filepath.IsLocal check, os.OpenRoot), but there are NO tests verifying these protections work. Missing test cases: (1) `lua_file: ''../../../etc/passwd''` should fail with clear error, (2) `lua_file: ''/etc/passwd''` (absolute path) should fail, (3) `lua_file: ''foo/../../../etc/passwd''` should fail (os.OpenRoot handles this but should be tested), (4) `lua_file: ''script.txt''` (non-.lua extension) should fail. Security-critical code MUST have explicit tests proving the protections work.'
severity: significant
resolution: 'Added TestLuaValidation_PathTraversal with tests for: path traversal with .., absolute paths, non-.lua extensions, and embedded traversal patterns. All security measures are now verified by tests.'
status: addressed
---
