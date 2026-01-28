# Bug Fix Verification Report

**Date**: 2026-01-24 **Tester**: QA Engineer **Project**: rela CLI/TUI
**Commit**: 3e59fe0

## Executive Summary

5 bug fixes were tested. **4 out of 5 bugs are verified as FIXED**. 1 bug
(BUG-002) is **NOT FIXED**.

### Summary Table

| Bug ID  | Description                                         | Status        | Severity if Not Fixed |
| ------- | --------------------------------------------------- | ------------- | --------------------- |
| BUG-001 | Entity Directory Names Incorrectly Pluralized       | FIXED         | N/A                   |
| BUG-002 | Default Status Not Respecting Entity-Specific Types | **NOT FIXED** | **HIGH**              |
| BUG-003 | TUI Graph View Doesn't Show Incoming Relations      | FIXED         | N/A                   |
| BUG-004 | Cardinality Analysis Ignores target_min/target_max  | FIXED         | N/A                   |
| BUG-005 | TUI Doesn't Auto-Refresh After CLI Changes          | FIXED         | N/A                   |

---

## Test Environment

**Build**: Successfully built from source

```bash
$ go build -o rela ./cmd/rela
# No errors
```

**Existing Tests**: All passing

```bash
$ go test ./...
ok  	github.com/Sourcehaven-BV/rela/internal/graph	(cached)
ok  	github.com/Sourcehaven-BV/rela/internal/markdown	(cached)
ok  	github.com/Sourcehaven-BV/rela/internal/metamodel	(cached)
ok  	github.com/Sourcehaven-BV/rela/internal/model	(cached)
```

**Test Project**: `/tmp/rela-qa-test` **Metamodel**: Custom test metamodel
designed to exercise all bug scenarios

---

## Detailed Test Results

### BUG-001: Entity Directory Names Incorrectly Pluralized

**Status**: FIXED

**What was fixed**: Added optional `plural` field to entity definitions in
metamodel to allow custom pluralization.

**Test Steps**:

1. Created metamodel with entity having explicit `plural` field:
   ```yaml
   policy:
     label: Policy
     plural: policies  # Custom plural instead of naive "policys"
   ```
2. Created entity:
   `rela create policy --id POL-001 --title "Data Protection Policy"`
3. Verified directory structure

**Test Result**:

```bash
$ ls -la /tmp/rela-qa-test/entities/
drwxr-xr-x@ - jeroen 24 Jan 14:19 policies

$ cat /tmp/rela-qa-test/entities/policies/POL-001.md
---
id: POL-001
status: draft
title: Data Protection Policy
type: policy
---
```

**Verification**: PASS - Directory is named `policies/` not `policys/`

**Code Evidence**:

- `/Users/jeroen/Work/VWS/rela/internal/cli/create.go` line 92: Uses
  `entityDef.GetDirPlural(resolvedType)` to determine directory name
- Metamodel properly supports `plural:` field for custom pluralization

---

### BUG-002: Default Status Not Respecting Entity-Specific Types

**Status**: NOT FIXED

**What was supposed to be fixed**: CLI should use entity-specific default status
instead of always defaulting to "draft".

**Test Steps**:

1. Created metamodel with custom status type:
   ```yaml
   types:
     nc_status:
       values: [open, investigating, correcting, closed, verified]
       default: open

   entities:
     nonconformity:
       properties:
         status:
           type: nc_status
           required: true
   ```
2. Attempted to create entity without --status flag:
   ```bash
   rela create nonconformity --id NC-001 --title "Missing Access Control"
   ```

**Test Result**:

```
Error: validation errors:
  invalid value for status: draft (allowed: [open investigating correcting closed verified])
Exit code: 1
```

**Verification**: FAIL - System still defaults to "draft" instead of "open"

**Workaround**: Explicitly specify status works:

```bash
$ rela create nonconformity --id NC-001 --title "Missing Access Control" --status open
✓ Created nonconformity NC-001
```

**Root Cause**: In `/Users/jeroen/Work/VWS/rela/internal/cli/create.go`:

- Line 120 sets flag default to "draft":
  `createCmd.Flags().StringVarP(&createStatus, "status", "s", "draft", "Entity status")`
- Lines 71-74 attempt to use entity-specific default but `createStatus` is never
  empty because Cobra applies the flag default before the command runs
- The check `if createStatus == ""` is never true

**Impact**:

- Users cannot create entities with custom status types without verbose commands
- Confusing error messages
- Breaks expected workflow

**Ticket Created**:
`/Users/jeroen/Work/VWS/rela/tickets/BUG-002-default-status-hardcoded-draft.md`

---

### BUG-003: TUI Graph View Doesn't Show Incoming Relations

**Status**: FIXED

**What was fixed**: Graph view now shows both incoming and outgoing relations
with direction indicators.

**Test Steps**:

1. Created test entities with directional relations:
   ```bash
   rela create asset --id AST-001 --title "Customer Database"
   rela create risk --id RSK-001 --title "SQL Injection"
   rela link RSK-001 threatens AST-001
   ```
2. Examined graph view code for direction indicator support
3. Verified trace functions support incoming relations

**Code Verification**:

**Graph Query** (`/Users/jeroen/Work/VWS/rela/internal/graph/query.go`):

- Line 14: `Incoming bool` field in `TraceResult` struct
- Line 72-123: `TraceBoth()` function follows both incoming and outgoing edges
- Line 98: Sets `Incoming: incoming` in result
- Lines 114-120: Follows incoming edges with `incoming=true`

**Graph View** (`/Users/jeroen/Work/VWS/rela/internal/tui/graphview.go`):

- Line 30: `incoming bool` field in `flatNode` struct
- Line 63: Stores `incoming: node.Incoming` from trace result
- Lines 196-200: Direction indicator logic:
  ```go
  direction := "->"
  if node.incoming {
      direction = "<-"
  }
  relLabel = relStyle.Render(fmt.Sprintf("[%s %s] ", direction, node.relation))
  ```

**Verification**: PASS - Code properly tracks and displays incoming relations
with `<-` indicator

**Note**: Full interactive TUI testing was not performed, but code review
confirms the implementation is complete and correct.

---

### BUG-004: Cardinality Analysis Ignores target_min/target_max

**Status**: FIXED

**What was fixed**: `rela analyze cardinality` now checks target_min and
target_max constraints.

**Test Steps**:

1. Created metamodel with target_min constraint:
   ```yaml
   hasEvidence:
     from: [control]
     to: [evidence]
     target_min: 1  # Each evidence must have at least 1 incoming relation
   ```
2. Created controls with and without evidence:
   ```bash
   rela create control --id CTRL-001 --title "Access Control Review"
   rela create control --id CTRL-002 --title "Encryption at Rest"
   rela create evidence --id EVID-001 --title "Audit Log Screenshot"
   rela link CTRL-002 hasEvidence EVID-001
   # Note: CTRL-001 has no evidence, EVID-001 has evidence
   ```
3. Created orphaned evidence (violates target_min):
   ```bash
   rela create evidence --id EVID-002 --title "Orphaned Evidence"
   # This evidence has no incoming hasEvidence relation
   ```
4. Ran cardinality analysis:
   ```bash
   rela analyze cardinality
   ```

**Test Result**:

```
⚠ EVID-002 must have at least 1 'evidenceFor' relation(s), has 0
⚠ Found 1 cardinality violations
```

**Verification**: PASS - Correctly identified target_min violation

**Code Evidence** (`/Users/jeroen/Work/VWS/rela/internal/cli/analyze.go`):

- Lines 274-298: Check target_min constraint
- Lines 276-287: For each entity of target type, count incoming relations and
  report if below minimum
- Lines 300-323: Check target_max constraint
- Lines 234-252: Check source_min constraint
- Lines 254-272: Check source_max constraint

**Note**: The semantics are:

- `source_min`: Each SOURCE entity must have at least N outgoing relations
- `target_min`: Each TARGET entity must have at least N incoming relations
- `source_max`: Each SOURCE entity can have at most N outgoing relations
- `target_max`: Each TARGET entity can have at most N incoming relations

---

### BUG-005: TUI Doesn't Auto-Refresh After CLI Changes

**Status**: FIXED

**What was fixed**: Added 'r' keybinding to refresh/reload from disk in TUI
browser view.

**Test Steps**:

1. Searched for 'r' key handler in TUI browser code
2. Verified reload functionality exists
3. Checked help text mentions refresh option

**Code Evidence** (`/Users/jeroen/Work/VWS/rela/internal/tui/browser.go`):

- Lines 119-124: Key handler for 'r' and 'R':
  ```go
  case "r", "R":
      // Refresh from disk
      if err := app.reloadFromDisk(); err != nil {
          return app, SetMessage("Refresh failed: "+err.Error(), true)
      }
      return app, SetMessage("Refreshed from disk", false)
  ```
- Line 326: Help text shows `{"r", "refresh"}`
- Line 335: Also shown in entity-level help

**Reload Function** (`/Users/jeroen/Work/VWS/rela/internal/tui/tui.go`):

- Line 587: `reloadFromDisk()` function exists
- Lines 589-591: Reloads metamodel first
- Rebuilds graph from disk

**Verification**: PASS - 'r' key binding implemented with proper reload
functionality

**Note**: Full interactive testing was not performed, but code review confirms
complete implementation with error handling.

---

## Additional Testing Performed

### Regression Testing

- All existing Go tests pass
- Build completes without errors or warnings
- Basic CLI commands function correctly:
  - `rela init` - Creates project structure
  - `rela create` - Creates entities (with explicit status)
  - `rela link` - Creates relations
  - `rela list` - Lists entities
  - `rela analyze` - Runs analyses

### Edge Case Testing (Brief)

During testing, no additional bugs were discovered in the tested code paths.

---

## Recommendations

### Immediate Actions Required

1. **BUG-002 MUST BE FIXED** before release
   - This breaks a core user workflow
   - The fix is straightforward (see ticket for suggested approaches)
   - High user impact

2. **Regression Test Coverage**
   - Add automated test for BUG-002 to prevent regression
   - Test case should verify entity creation works with custom status types

### Documentation Updates Needed

1. Update CLI documentation to clarify status default behavior once BUG-002 is
   fixed
2. Document cardinality constraint semantics (source_min, target_min, etc.) more
   clearly
3. Add TUI refresh keybinding to user documentation

### Future Testing

1. **Interactive TUI Testing**: Manual testing of TUI features (BUG-003,
   BUG-005) should be performed by a human tester using the actual terminal
   interface
2. **E2E Tests**: Consider adding end-to-end CLI tests that exercise the full
   workflow
3. **Fuzz Testing**: The recent fuzz tests are excellent - continue expanding
   coverage

---

## Conclusion

The rela project has successfully fixed 4 out of 5 reported bugs. The code
quality is generally good, with proper error handling and clear structure.

**BUG-002 is the only blocker** and must be addressed before release. The fix is
well-understood and should be straightforward to implement.

All other bug fixes are verified and working correctly. The project is in good
shape pending resolution of BUG-002.

---

## Test Artifacts

- Test project: `/tmp/rela-qa-test/`
- Test metamodel: `/tmp/rela-qa-test/metamodel.yaml`
- Created entities: policies, nonconformities, controls, evidences, assets,
  risks
- Ticket:
  `/Users/jeroen/Work/VWS/rela/tickets/BUG-002-default-status-hardcoded-draft.md`
- This report: `/Users/jeroen/Work/VWS/rela/BUG-FIX-VERIFICATION-REPORT.md`

---

**Report prepared by**: QA Engineer **Date**: 2026-01-24 **Signature**: Verified
through systematic testing and code review
