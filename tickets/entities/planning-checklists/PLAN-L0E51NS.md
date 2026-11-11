---
id: PLAN-L0E51NS
type: planning-checklist
title: 'Planning: Remote pull: refuse symlinked credential files and hide unreadable conflict rows'
started: "2026-11-09"
completed: "2026-11-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the declared credential file read, and the conflict issues of the
entity pull. Out: other report paths of the run that name ledger rows (the gone
path), reported as follow-ups if the reviewers find them.

**Acceptance Criteria:**
1. A declared credential file that is a symlink to a file without a remote credential name is refused, and its content is not read. Test: `TestCredential_ReadDeclaredSymlink`.
2. A symlink to a file with a remote credential name (a Kubernetes secret mount) is read. Same test.
3. A conflict on a local row the puller cannot read has no local id and no type in the report. Test: `TestRun_ConflictHidesUnreadableLocalRow`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small security fix)
- [x] ~~Searched for existing libraries~~ (N/A: standard library os/filepath)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: small fix)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The gated reader is `lua.ReadDeps.VisibleReader`
(`visibility.ScriptReader`), already in the scheduled Lua deps the pull uses.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `Credential.ReadDeclared` resolves symlinks and requires the resolved base name to be a remote credential name, then Lstat, Open and SameFile. Refusing every symlink was rejected because it breaks Kubernetes secret mounts.
2. `remotesync.Deps.Gate` (consumer-side `ReadGate`) is checked before a located local row is reported or merged. A hidden row is a conflict without a local id.

**Files to modify:** internal/reconclient/httpremote/credential.go,
internal/remotesync/run.go, internal/scheduler/remotes.go, and their tests.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** remotes.yaml names the credential file; the
resolved name must match the remote credential allowlist prefix.

**Security-Sensitive Operations:** Reading a credential file and sending it to a
remote; reporting local rows found through a raw read.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria.

**Edge Cases:** Symlink chains, a directory with a credential name, a readable
conflicting row (still named).

**Negative Tests:** Symlink to another secret is refused; hidden row is not
named.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A missing gated reader would stop pulls; remoteDeps refuses with a
clear error.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: behavior fix, no new configuration)
- [x] ~~Docs-checklist will be created~~ (N/A: chore)

**Documentation Impact:** N/A

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the brief prescribed the fix; reviewed by cranky-code-reviewer and rela-security-reviewer)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
