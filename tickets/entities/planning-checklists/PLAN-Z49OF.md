---
id: PLAN-Z49OF
type: planning-checklist
title: 'Planning: Introduce RootedFS type and pilot on state.FSKV'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (in):**

- New concrete type `storage.RootedFS` that wraps any `storage.FS` and binds it to a validated root directory.
- `RootedFS` exposes the same method shapes as `storage.FS` (except `Getwd`), but interprets the `path`-like argument as a *key* relative to the root.
- Single `resolve(key) (string, error)` method is the path-validation barrier (matches the current `state.validateKey` rule set).
- Migration of `state.FSKV` to use `*RootedFS` internally.
- Full unit test coverage on `RootedFS`.

**Scope (out):**

- `fsstore` migration (TKT-3TA1H).
- Arch lint enforcement (TKT-K3YYE).
- Package split into `storage/raw` + `storage/rooted` (TKT-REC7P).
- `Getwd()` — doesn't fit the keyed-access model. `RootedFS` intentionally does not expose it.
- Changing `storage.FS` interface or semantics.

**Acceptance Criteria:**

1. **AC1**: `storage.RootedFS` exists with constructor `NewRootedFS(fs FS, root string) (*RootedFS, error)`. Constructor cleans/absolutizes `root` and returns error on empty root.
   - *Test*: `TestNewRootedFS_CleansRoot`, `TestNewRootedFS_RejectsEmptyRoot`.
2. **AC2**: `resolve(key)` rejects empty, absolute, backslash-containing, control-char-containing, `..`-segment, empty-segment, drive-letter keys. Accepts nested keys like `a/b.json`. Returns `filepath.Join(root, key)` for valid keys.
   - *Test*: `TestRootedFS_Resolve_Accepts`, `TestRootedFS_Resolve_Rejects` (table-driven, mirrors existing `TestValidateKey_*` coverage).
3. **AC3**: All FS method shapes supported by `RootedFS` (ReadFile, WriteFile, Remove, Rename, Stat, MkdirAll, ReadDir, Walk, Open) delegate to the underlying FS with the resolved path. Rename validates both args. `Walk(key, fn)` walks a subtree; separate `WalkAll(fn)` walks from the root.
   - *Test*: `TestRootedFS_WriteFileDelegates`, `TestRootedFS_ReadFileDelegates`, `TestRootedFS_Rename_ValidatesBothKeys`, `TestRootedFS_WalkAll_FromRoot`, etc.
4. **AC4**: `RootedFS.Walk` and `RootedFS.WalkAll` call the user's `fs.WalkDirFunc` with a *key* (root-relative path), not the absolute resolved path. Ensures callers don't accidentally see or use the raw backing path.
   - *Test*: `TestRootedFS_Walk_ReturnsKeys`, `TestRootedFS_WalkAll_ReturnsKeys`.
5. **AC5**: `state.FSKV` uses `*RootedFS` internally. `validateKey` and the explicit `filepath.Join` in `state.go` are gone.
   - *Test*: Existing `TestValidateKey_*` tests move to `storage/rooted_test.go` (renamed `TestRootedFS_Resolve_*`). State tests pass unchanged.
6. **AC6**: `just ci` is green.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `io/fs.Sub` (stdlib): provides a rooted sub-filesystem above `fs.FS`, but `fs.FS` is read-only and rela's `storage.FS` is read+write+rename+mkdir. Not a fit, but same design pattern.
- `cdproto/securefs` and similar — specific to Chrome DevTools, not applicable.
- Existing in-codebase pattern: `state.validateKey` (`internal/state/state.go:67-91`) already implements the exact validation rule set we want. This ticket's `resolve` lifts it verbatim.
- `fsstore` uses entity/relation ID sanitization upstream (in `storeutil.ValidateID`) before constructing paths — different layer, not a validator we can share.
- Prior related work: TKT-020 (graph → storage dep removed), TKT-031 (CLI storage dep removed), FEAT-022 (arch lint boundary enforcement). These set the pattern for "make architectural contracts explicit and enforce via lint." RootedFS extends that pattern to path safety.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Create `internal/storage/rooted.go`:

```go
package storage

import (
    "errors"
    "io"
    "io/fs"
    "os"
    "path/filepath"
    "strings"
)

// RootedFS is an FS bound to a validated root directory. Path-like
// arguments to its methods are interpreted as KEYS relative to the
// root. Every call runs keys through resolve(), which is the single
// path-validation barrier.
//
// RootedFS deliberately does NOT implement storage.FS: callers that
// accept *RootedFS get a compile-time claim that paths have been
// validated. Callers that accept storage.FS continue to see raw,
// caller-validated paths.
//
// Getwd is omitted — it does not fit the keyed-access model.
type RootedFS struct {
    fs   FS
    root string
}

func NewRootedFS(fs FS, root string) (*RootedFS, error) {
    if root == "" {
        return nil, errors.New("storage: RootedFS root must not be empty")
    }
    abs, err := filepath.Abs(root)
    if err != nil {
        return nil, err
    }
    return &RootedFS{fs: fs, root: filepath.Clean(abs)}, nil
}

// resolve validates a key and joins it with the root. Rules mirror
// state.validateKey exactly.
func (r *RootedFS) resolve(key string) (string, error) { /* ... */ }

// Root returns the absolute, cleaned root directory. Used by tests
// and by the few wiring sites that need the physical location.
func (r *RootedFS) Root() string { return r.root }

// ReadFile, WriteFile, Remove, Rename, Stat, MkdirAll, ReadDir,
// Walk, Open — same shape as FS, but take keys.
//
// Walk(key, fn) walks a subtree; WalkAll(fn) walks the whole rooted
// tree (no key argument). Both rewrite the callback's path argument
// from absolute → key before invoking the user's fs.WalkDirFunc, so
// callers never see the backing filesystem location.
```

**Validation rules in `resolve` (copied from `state.validateKey`):**

| Reject | Reason |
|---|---|
| `""` | empty |
| any `r < 0x20 \|\| r == 0x7f` | control character |
| contains `\\` | backslash |
| starts with `/` | absolute |
| any segment is `""`, `.`, or `..` | traversal/empty |
| `len(name) >= 2 && name[1] == ':'` | Windows drive letter |

Accept: single-segment keys, nested keys with `/` separators.

**Walk callback remapping:**

```go
func (r *RootedFS) Walk(key string, fn fs.WalkDirFunc) error {
    full, err := r.resolve(key)
    if err != nil { return err }
    return r.fs.Walk(full, func(path string, d fs.DirEntry, err error) error {
        rel, relErr := filepath.Rel(r.root, path)
        if relErr != nil { rel = path } // defensive
        return fn(filepath.ToSlash(rel), d, err)
    })
}
```

**Files to modify:**

- `internal/storage/rooted.go` (new)
- `internal/storage/rooted_test.go` (new)
- `internal/state/state.go` — replace `storage.FS + root + validateKey` with `*storage.RootedFS`
- `internal/state/state_test.go` — remove `TestValidateKey_*` (covered by `rooted_test.go`); keep FSKV integration tests.

**Wiring for `state.FSKV`:**

Before:
```go
func NewFSKV(fs storage.FS, dir string) *FSKV
```

After:
```go
func NewFSKV(rfs *storage.RootedFS) *FSKV
```

Callers of `NewFSKV` need updating. Let me audit call sites during
implementation.

**Alternatives considered:**

1. **`SafePath` type instead of `RootedFS` facade.** Rejected: requires changing every `FS` method signature; ripples through MemFS, OsFS, SafeFS, and every decorator. Higher blast radius, weaker containment.
2. **Add `resolve` as a free function in `storage`.** Rejected: doesn't bind a root, so every caller still writes `resolve(key); s.fs.WriteFile(filepath.Join(root, resolved)...)`. No centralization win.
3. **Extend `storage.FS` with a root-aware variant method.** Rejected: bloats the interface; doesn't solve the "which root?" question; can't be enforced via arch lint without a package split anyway.

**Dependencies:**

- Uses only `io`, `io/fs`, `os`, `path/filepath`, `errors`, `strings` from stdlib.
- Depends on `storage.FS` (same package, already exists).
- No new vendor deps.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Input | Validation |
|---|---|---|
| `state.FSKV` callers | key string (e.g., `"cache.json"`, `"documents/render-abc.html"`) | `resolve()` — reject empty, `..`, absolute, backslash, control chars, drive letters |
| `NewRootedFS` caller | root string (from wiring layer, not user input) | `filepath.Abs` + `filepath.Clean`; reject empty |

**Security-Sensitive Operations:**

- **File I/O at the root**: every write/read ends up calling the underlying `storage.FS` (which in prod is `SafeFS(OsFS)`) with a resolved absolute path. The resolved path is structurally inside the root by construction (join of cleaned root + validated key with no `..`).
- **Error messages**: errors from `resolve` describe the rule violated (empty, traversal, etc.) but do not echo the key's contents — consistent with `state.validateKey` today.
- **Walk path remapping**: the callback receives keys, never absolute paths. Prevents accidentally leaking the backing filesystem location to callers (e.g., into error messages surfaced to the user).

**Is this actually rooted?** The combination of (a) root being
`filepath.Clean(filepath.Abs(root))`, (b) key rejecting absolute paths,
backslashes, `..`, empty segments, and (c) `filepath.Join(root, key)` means the
joined result is always a descendant of root. `filepath.Clean` in `Join` would
normalize any remaining `.` segments; since `..` is already rejected, no escape
is possible on POSIX or Windows. Test coverage will include an explicit
`TestRootedFS_ResolvedPath_StaysInsideRoot` case.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (AC → tests):**

- **AC1**: `TestNewRootedFS_CleansRoot`, `TestNewRootedFS_RejectsEmptyRoot`, `TestNewRootedFS_ResolvesRelativeRoot`.
- **AC2**: `TestRootedFS_Resolve_Accepts` (table: valid single-segment, valid nested, valid dot-in-filename). `TestRootedFS_Resolve_Rejects` (table: empty, `..`, `.`, `/abs`, `a\b`, `../esc`, `sub/../esc`, NUL, `\x01`, `a//b`, `c:file`).
- **AC3**: For each of ReadFile, WriteFile, Remove, Rename, Stat, MkdirAll, ReadDir, Open — test with `MemFS` as underlying; verify (a) valid key delegates correctly, (b) invalid key returns error without touching underlying FS. Use a spy wrapping MemFS to assert non-call on invalid keys.
- **AC4**: `TestRootedFS_Walk_ReturnsKeys` — populate MemFS with `/root/a.txt`, `/root/sub/b.txt`, walk from `"."`, assert callback receives `"a.txt"`, `"sub"`, `"sub/b.txt"` (not absolute paths).
- **AC5**: `TestFSKV_Put_Get_RoundTrip`, `TestFSKV_Put_RejectsInvalidKey` (moved/rewritten).
- **AC6**: CI gate.

**Edge Cases:**

- Empty key → rejected.
- Single `.` → rejected (matches current validator).
- Key with embedded `/./` → rejected (empty segment after split on `/`)? Actually `strings.Split("a/./b", "/")` gives `[a, ., b]`, and `.` is rejected. Good.
- Unicode in key → allowed (no control chars, no separators). Document as accepted.
- Very long key → no explicit length cap; let underlying FS return the OS error.
- Concurrent calls → `RootedFS` is stateless after construction (root + fs are immutable), so inherently concurrency-safe. Inherits the underlying FS's concurrency semantics.
- Rename where source is valid but destination is invalid → return error without touching FS.

**Negative Tests:**

Each invalid input from the rejection table above gets its own test case. Tests
assert:
1. Method returns a non-nil error.
2. Error message mentions the specific rule violated.
3. Underlying FS is NOT called (via spy).

**Integration approach:**

- Unit tests use `MemFS` as the underlying FS for speed and determinism.
- `state.FSKV` integration tests (existing) continue to work with the migrated API and exercise the full stack.
- No separate end-to-end test needed for this ticket — it's a refactor of a validated path that already has coverage.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `filepath.Rel` in `Walk` callback fails on unusual paths (symlinks, mount boundaries) | Low | Low | Defensive fallback to absolute path; test with symlinks in a follow-up if issue observed. Current callers use in-memory or known-local paths. |
| `NewFSKV` API change breaks callers | Certain | Low | Audit during implementation; there's only a handful of call sites. Update all at once in same PR. |
| `state.FSKV` Reader tests assume specific error shapes (`os.IsNotExist`) | Medium | Low | Keep error wrapping consistent — `RootedFS` methods return the underlying FS error verbatim on valid keys. Only validation errors are new. |
| Subtle semantic change: `validateKey` was called BEFORE `filepath.Join`; `resolve` does both. Any caller that constructed paths outside `FSKV` won't be migrated yet. | Low | Low | This ticket doesn't migrate fsstore or other callers; they stay on `storage.FS`. No behavior change for them. |
| "Walk the whole tree" — how to express without a magic-string key? | Medium | Low | Add a dedicated `WalkAll(fn)` method for walking from root. `Walk(key, fn)` rejects `""` and `.` like every other method. No asymmetry. |

**Walk-root decision:** `Walk(key, fn)` validates `key` with the same rules as
every other method (rejects `""`, `.`, `..`). A separate `WalkAll(fn
fs.WalkDirFunc)` method walks from the root — no key argument. Callers that want
"walk everything" say `rfs.WalkAll(fn)`; callers that want a subtree say
`rfs.Walk("sub", fn)`. Naming the intent at call sites is clearer than
overloading `Walk` with a magic empty string.

**Effort estimate: `s`.** New file ~150 LOC + ~250 LOC of tests. `state.FSKV`
migration is ~20 LOC of diff. No cross-package changes except `state`.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

- [x] ~~User guide / reference docs~~ (N/A: internal type)
- [x] ~~CLI help text~~ (N/A: no CLI changes)
- [x] ~~CLAUDE.md~~ (N/A: will update in a follow-up ticket once the pattern is established across more callers; premature for a single-caller pilot)
- [x] ~~README.md~~ (N/A: internal type, no project-level change)
- [x] ~~API docs~~ (N/A: no public HTTP/CLI API surface)

Package-level doc comment in `internal/storage/rooted.go` will explain the
pattern.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design was iterated interactively with the user across 11 conversation turns before planning — the facade-vs-type design, Walk/WalkAll split, and arch-lint strategy were all settled in that dialogue. Formal `/design-review` would have re-litigated agreed decisions.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above. The post-implementation `/code-review` surfaced 6 significant findings (missed in the interactive design dialogue) which have all been addressed — see REV-I4AEM.)

**Design Review Findings:** Addressed via post-implementation `/code-review` instead. 15 review-responses linked to TKT-0M8PM. All significant findings status=addressed.
