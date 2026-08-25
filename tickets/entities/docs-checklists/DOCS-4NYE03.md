---
id: DOCS-4NYE03
type: docs-checklist
title: 'Docs: Investigate SQLite as a third store backend (TKT-TWIO11)'
status: done
---

<!-- @managed: claude-workflow v1 -->

**Investigation ticket — no user-facing change shipped.** The deliverable is
DEC-LFSYNY plus the measurements in `internal/store/sqlitespike/RESULTS.md`.
Items are marked N/A with a reason rather than silently ticked.

## Code Documentation

- [x] Package doc written — `internal/store/sqlitespike` opens by stating it is
a THROWAWAY that answers one question, is not a backend, and lives on a branch
that is never merged. That framing is the most important thing a reader needs.
- [x] Non-obvious decisions explained at the point of use — the DSN-vs-`db.Exec`
PRAGMA finding is documented in `Open` where the mistake would be made again,
not in a separate note; `verifyBusyTimeout` explains why it pins two
connections; the nested-`Tx` comment cites `pgstore/tx.go:61-63` and names the
self-deadlock it prevents.
- [x] Unimplemented methods say why — `errUnsupported` exists precisely so a
spike cannot emit confident wrong evidence from a silent stub.
- [x] Reproduction instructions — `RESULTS.md` ends with runnable commands,
including the warning that arm C hangs by design and must be run under
`timeout`.

## Project Documentation

- [x] ~~`docs/` guide~~ (N/A: nothing ships. A `docs/sqlite-backend.md`
paralleling `postgres-backend.md` is scoped into TKT-G91TBK, including the
single-writer constraint and the git-versus-version-history trade-off.)
- [x] ~~`CLAUDE.md` backend table row + build-tag rules~~ (N/A: no backend
exists yet. Scoped into TKT-G91TBK, together with the CI backend-isolation
assertions.)
- [x] ~~`README.md` backend selection guidance~~ (N/A: same reason.)
- [x] Decision recorded where the project looks for decisions — DEC-LFSYNY,
with measurements rather than adjectives, and an explicit "not licensed by this
decision" section so it cannot be read as approving multi-process SQLite or a
desktop-default switch.

## Knowledge Capture

- [x] Findings that would otherwise be rediscovered are written into the
follow-up ticket, not left in a branch. TKT-G91TBK carries a "do not rediscover"
list: DSN PRAGMAs, `BEGIN IMMEDIATE`, never shrink the pool to serialize,
nested-`Tx` shape, and `storeutil` as the validity oracle.
- [x] The unmeasured gap is documented as unmeasured — arm F (WAL on network
filesystems) is recorded in RESULTS.md, the review checklist and the ticket body
as **assumed, not verified**, with the ready-made probe command and a concrete
startup-guard mitigation in TKT-G91TBK.
- [x] Design-review value captured — PLAN-ZLVJC3 records that C1 (module layout
could not compile) and C3 (primary arm made the risk unobservable) were caught
before implementation, and that C3 was then confirmed in execution when arm C
hung.
