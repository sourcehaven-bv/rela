---
paths:
  - "internal/check/**"
  - "internal/metamodel/checkers.go"
  - "internal/cli/analyze_prose.go"
---

# Prose checks (`internal/check`)

The `checkers:` map in the metamodel registers external prose checkers (see
`docs/checkers.md`). The engine runs them through `internal/cmdexec` with
`PurposeCheck`, its own read-path list (`RELA_SANDBOX_CHECK_READ_PATHS`).
Rules for new code:

- **Findings are advisory and never on the write path.** No save, create or
  automation waits for a checker or fails because of one.
- **A request may name no command, flag or path.** Checkers come from the
  metamodel only. The entity type reaches the command as the `<type>/<id>.md` input
  name and `{type}`, and the metamodel allowlists type names whenever
  checkers are declared; cmdexec checks the input name again.
- **rela does not manage checker config.** Styles and `.vale.ini` are the
  operator's, installed outside the project and listed in the check read
  paths. Do not add copying or staging of project directories.
- **Build the engine once and share it.** It owns the bounded pool, like the
  transform engine; a per-request engine bounds nothing.
- **Analyze runs checkers only when asked.** Each entity costs a process, so
  `analyze all` needs `--prose`. Do not add prose to an analyze path that
  every reader or routine run triggers.
- **One finding shape.** Parsers map a tool's output to `check.Finding`:
  1-based line and column, column and length in code points, `Match` taken
  from the body. A position that does not fit is cleared, not dropped.
