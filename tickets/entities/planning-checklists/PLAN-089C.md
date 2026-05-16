---
id: PLAN-089C
type: planning-checklist
title: 'Planning: Lift workspace analysis / attachment / rename-type facades to dedicated packages'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented

**Problem.** `internal/workspace/attachment.go` (~130 LOC) implements
`AttachFile` and `ListAttachments` — CLI-shaped operations that live on
`*workspace.Workspace`. CLI's `cliAnalyze` interface returns
`*workspace.AttachResult` / `[]workspace.AttachmentInfo`, leaking workspace
types across the consumer boundary. After TKT-0SP1 dropped the package globals,
this is the next gap to close.

**Scope (in):**
- New `internal/attachment` package with `Service` constructor.
- Methods `Attach` / `List` on the service.
- Types `AttachmentInfo` / `AttachResult` move with the methods.
- `cliAnalyze` interface returns the new types.
- `cliServices` forwarders consume the new service.
- `internal/workspace/attachment.go` + tests DELETED.
- `internal/workspace/attachment_filename_test.go` (helpers test) moves with `contentTypeForName`.
- `.go-arch-lint.yml`: add `attachment` component; cli gains the dep; workspace drops the related deps if no longer needed.

**Scope (out):**
- RenameEntityType (TKT-04YA) and analysis (TKT-B01S) lifts.
- Attachment storage semantics (fsstore's `AttachFile` method is unchanged).
- Method renaming (keep `AttachFile`/`ListAttachments` shape; just move them).

**Acceptance Criteria:**
1. `internal/attachment` package exists with `Service`, `New`, `Attach`, `List`, `AttachmentInfo`, `AttachResult`.
2. `internal/workspace/attachment*.go` deleted.
3. `internal/cli/cli_wiring.go` uses `attachment.AttachResult` / `attachment.AttachmentInfo` types in `cliAnalyze` signatures.
4. `internal/cli/attach.go` / `attachments.go` compile unchanged (they only see types via the interface).
5. `just test -race`, `just lint`, `just arch-lint`, `just ci` all pass.

## Research

- [x] Pattern established by TKT-LCTG (lifted search-related types into search package) and TKT-Q1JT (Observer wiring) — same shape applies here.
- [x] `attachment.go` deps: Store + Meta + EntityManager (for `UpdateEntity` to persist the property change). All already available in `cliServices`.

## Approach

### Step 1 — Create `internal/attachment` package

- `internal/attachment/attachment.go`: copy types + `Service` with `New(deps)` constructor.
- `internal/attachment/filename.go`: copy `findFileProperty` + `contentTypeForName` helpers.
- `internal/attachment/attachment_test.go` / `filename_test.go`: copy from workspace, repackage.

### Step 2 — Wire into CLI

- `cli_wiring.go`: `cliAnalyze.AttachFile` / `ListAttachments` signatures return `*attachment.AttachResult` / `[]attachment.AttachmentInfo`.
- `cliServices` grows an `attach *attachment.Service` field; forwarders call into it.
- `newCLIServices` constructs the service from workspace's Store/Meta/EntityManager.

### Step 3 — Delete from workspace

- Delete `internal/workspace/attachment.go` and `_test.go` files.

### Step 4 — arch-lint

- Add `attachment: { in: internal/attachment }` component.
- `attachment.mayDependOn`: entitymanager, metamodel, store.
- `cli.mayDependOn`: add `attachment`.

## Risks

- Test migration: 250 LOC of tests across two files. Mechanical.
- Interface signature change in `cliAnalyze` ripples to nothing — only the wiring + subcommand handlers, which already use the interface. Type assertions stay the same.

**Effort:** S — single mechanical lift, ~130 LOC code + ~250 LOC tests moved.
