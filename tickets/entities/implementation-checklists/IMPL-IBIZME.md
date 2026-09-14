---
id: IMPL-IBIZME
type: implementation-checklist
title: 'Implementation: classify the unreached set with reasoned coverage-ignore directives'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Code written
- [x] Follows existing patterns
- [x] No unrelated changes

**What changed.** 260 `// coverage-ignore*` directives added across 109 Go
files; `COVERAGE-HONESTY-REPORT.md` regenerated; `scripts/coverage-generous.sh`
dropped as superseded.

**Rebase.** 32 files conflicted. Each was resolved by taking the upstream code
verbatim and reattaching the directive to the construct it classified. Where the
construct was gone, the directive was dropped rather than moved to a neighbour.

**Dropped: 19 directives whose construct no longer exists.** By cause:

- 8 in `cmd/rela-desktop/main.go` — Wails v3 replaced the `runtime.*` API
  (`WindowSetTitle`, `Quit`) and the `menu.CallbackData` handler signatures the
  directives sat on.
- 13 elsewhere — the annotated `if err != nil` guard or early-return was
  restructured upstream (`internal/search/bleveindex` ×3,
  `internal/automation/template.go` ×2, and one each in `acl/source.go`,
  `appbuild`, `appbuild_fs`, `attachment/cmdrunner`, `dataentry/api_v1`,
  `lua/runtime`, `output/output`, `store/fsstore/fsstore`).

17 orphaned `-end` markers left by those drops were removed; `-start`/`-end`
pairs are balanced at 161/161.

**Dropped: 3 more as factually stale.** Verified reached in the merged profile,
so the "unreachable" reason no longer held: `internal/config/config.go`,
`internal/predicate/eval.go`, `internal/store/fsstore/watcher.go`.

22 dropped in total (2 single-line, 20 block), 260 surviving.

The branch carried 282 directives (104 single-line + 178 block `-start`; the
178 matching `-end` markers are closers, not annotations). 22 were dropped,
leaving 260 net-new directives against develop, which itself carries 52.

**Dialect conversion.** All surviving directives converted from
`//scupper:ignore` to `// coverage-ignore`. Verified empirically first, on a
throwaway module: under `-d coverage-ignore` a `scupper:ignore` comment is not
honoured and its block is reported unreached; under the default the reverse
holds. The two spellings are mutually exclusive, so the branch's directives
would have been inert as written.

## Quality

- [x] Tests written/updated
- [x] Edge cases handled
- [x] Error handling complete

No test changes: this change adds comments and deletes a script, and alters no
runtime behaviour. `git diff --stat` over `*.go` shows insertions of comment
lines only, plus the conflict resolutions that restore upstream code.

Correctness was established against the coverage profile instead — see the
review checklist for the counts.
