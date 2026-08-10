---
id: REV-AZU8G0
type: review-checklist
title: 'Review: Unify metamodel->predicate type adapter + filter->predicate transpiler (TKT-7EJK4 phase 2a)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Scope:** phase 2a (adapter unification + transpiler). Slices 3+4 ->
TKT-J4IR1G.

## Automated Checks

- [x] Tests pass (`-race` green on all changed pkgs + dependents; full `go test ./...` clean except unrelated headless-Chrome `docscapture`)
- [x] Lint clean (golangci-lint 0 issues on changed packages)
- [x] Coverage maintained (predicatefns/affordances well above floor)

## Code Review

- [x] `/design-review` run (adversarial, pre-implementation)
- [x] All critical review-responses addressed (RR-WHMVLW, RR-782ULH, RR-TQEHO4)
- [x] All significant review-responses addressed for this scope (RR-IRV2WJ, RR-NKWJS6, RR-23W88J); RR-2Y851X/RR-02P03I re-scoped to TKT-J4IR1G
- [x] Self-reviewed diff for unrelated changes (only predicate/predicatefns/affordances + arch-lint + tickets)

**Review Responses:** RR-WHMVLW, RR-782ULH, RR-TQEHO4 (critical, addressed);
RR-IRV2WJ, RR-NKWJS6, RR-23W88J (significant, addressed).

## Acceptance Verification

- [x] AC1 (adapter unified) PASS — affordances on shared ScalarType, time.Time+string date binder, int coercion preserved; distinguishing tests for the semantic change
- [x] AC2 (transpiler parity) PASS — cross-engine matrix identical to filter.Match; unsupported cases error
- [x] AC5 (no eval-time I/O) PASS — arch_test green; date parse at compile/bind
- [x] ~~AC3 (--filter) / AC4 (automation migration)~~ (moved to TKT-J4IR1G phase 2b)

## Documentation

- [x] ~~Docs-checklist~~ (N/A: `kind=refactor`, no user-facing surface in phase 2a — no `--filter` flag or syntax change yet; godoc on new exported funcs. User docs land with phase 2b's `--filter`.)

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs
- [x] Ready for another developer (godoc on ScalarType/ScalarTypeForProp/FromFilter/len)

## Pull Request

- [x] Run `/pr` (this step)
- [x] All CI checks pass (monitored after push)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1305
