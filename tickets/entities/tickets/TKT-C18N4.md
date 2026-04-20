---
id: TKT-C18N4
type: ticket
title: Promote demo scripts to acceptance tests (just + CI)
kind: enhancement
priority: medium
effort: s
tags:
    - tech-debt
status: ready
---

## Description

The existing `demos/*/demo.sh` scripts (starting with
`demos/encryption/demo.sh`) are already end-to-end acceptance tests for their
feature area — they build the binary, exercise the CLI, and assert invariants
via exit code + grep. Right now nobody runs them automatically, so regressions
slip through: a CLI flag rename, a subtly-changed output format, a
filesystem-layout change all break the demo silently until someone manually
re-runs it.

Promote them to first-class acceptance tests: one `just` target runs all demos,
and CI invokes that target on every PR.

## Scope

**In scope:**

- Add `just demo <name>` target that runs a single demo script.
- Add `just demos-test` target that runs every `demos/*/demo.sh`, failing fast on the first non-zero exit. Build `bin/rela` once up front.
- Add `demos-test` as a job to `.github/workflows/ci.yml` (runs on PR and push-to-develop). Uses the same CI cadence as existing jobs.
- Document in `CLAUDE.md` the convention "every CLI feature ships a `demos/<feature>/demo.sh` that's both a tutorial and an acceptance test."
- Audit existing demos and make sure each is a pure acceptance test (`set -euo pipefail`, exits non-zero on any invariant break, no interactive prompts).

**Out of scope:**

- Rewriting demos as Go tests (shell is the right idiom).
- Windows support (demos are Linux/macOS only; matches rest of project).
- Parallelizing demos (each is ~5s; serial is fine).
- Harvesting "missing" demos for features that don't have one yet — that's a follow-up audit ticket.

## Design

```just
# Run a single demo by name (e.g. `just demo encryption`)
demo name:
    @bash demos/{{name}}/demo.sh

# Build rela once then run every demo as an acceptance test.
# Fails fast on the first non-zero exit.
demos-test: build-cli
    @set -e; for d in demos/*/demo.sh; do \
        echo ""; echo "=== $d ==="; \
        bash "$d"; \
    done
    @echo ""; echo "All demos passed."
```

CI job (addition to `ci.yml`, mirroring the Build job shape):

```yaml
demos:
  name: Demos
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: { go-version: '1.26' }
    - name: Run demos
      run: just demos-test
```

## Why not wrap in `go test`

- Demos take ~5s each. `go test ./...` is on the hot path for every dev loop; adding subprocess-based e2e would tax it unnecessarily.
- `go test` coverage measurement gets muddled by subprocess-driven scenarios. The coverage ratchet already has tight tolerances.
- Shell is the natural idiom for demo scripts (pipe-fail, grep-assert, colored output). Rewriting in Go would lose readability.
- Demos serve double duty as user-facing tutorials. Keeping them as runnable `.sh` files means a user can `bash demos/encryption/demo.sh` to try the feature.

## Acceptance criteria

1. `just demo encryption` runs the encryption demo and exits 0.
2. `just demos-test` runs every `demos/*/demo.sh` and reports pass/fail per demo.
3. `demos-test` CI job runs on every PR and push to `develop`.
4. A deliberately broken demo (e.g. wrong flag name) fails `just demos-test` with a clear error.
5. `CLAUDE.md` documents the "new CLI features ship a demo" convention.
6. Every existing `demos/*/demo.sh` passes `demos-test` on first run (no latent breakage).

## Follow-ups (separate tickets)

- Audit which features lack demos and prioritize writing them (flow, scheduler, migrations, MCP).
- Consider a `demos-test --record` mode that captures demo output as a golden file for snapshot diffing on future changes.
