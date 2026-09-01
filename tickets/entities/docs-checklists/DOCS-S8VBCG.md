---
id: DOCS-S8VBCG
type: docs-checklist
title: 'Docs: Capability-gate mail.send'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`Capabilities.Mail` is documented alongside `HTTP` and `AI`, explaining why the
http/ai reasoning transfers here even though mail's transport is operator
config.

The circularity in `ScriptCapabilities.toLua()` is written down: mail's own
send-script runtime hard-wires `Mail: true`, because the runtime whose entire
job is sending mail is the one place the gate cannot apply. Without that note it
reads as an oversight, and the next person "fixes" it.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: applies the existing
capability-gating pattern to a third binding)
- [x] docs/ updated for changed behaviour — see below
- [x] ~~Architecture docs updated~~ (N/A: no boundary or wiring change)

Two guides, and one of them had to be REWRITTEN rather than extended.

**`GUIDE-lua-scripting.md`** previously stated, as published guidance:

> unlike `http` and `ai`, whose absence *is* the capability gate, `mail.send` is
> not a capability a script holds

That is now false. Leaving it and adding a note elsewhere would have left the
docs self-contradicting, with the wrong half stated more confidently. The
capability section heading also changes, and it carries an anchor other pages
link to, so the cross-reference was updated with it.

**`GUIDE-mail.md`** needed a disambiguation rather than a correction. It already
documented a `capabilities:` block — but that is `mail.yaml`'s grant for the
SEND SCRIPT, a different thing from a script-side `mail:` grant. Two similarly
named things one page apart is exactly how an operator grants the wrong one, so
they are now named distinctly.

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLI reference updated~~ (N/A: no command or flag)
- [x] ~~API docs updated~~ (N/A: no HTTP surface change)

## Rationale for N/A

The user-visible change is a config key an operator adds, documented in the
guides above. No route, request or response shape moves.

Worth recording, because it is the part most likely to be misread as an
oversight: **this is a BREAKING change and the docs say so plainly.** Existing
projects that send mail from Lua must add the grant or their sends will be
denied. Backwards compatibility was explicitly waived by the project owner, on
the reasoning that a security default which ships "off for now" is a security
default nobody ever turns on.

The boot warning is the mitigation, and it is documented as such: an operator
upgrading learns from a log line naming the affected actions, rather than from a
scheduled digest that silently stopped arriving. A migration note in a changelog
would reach only the operators who read changelogs; the warning reaches the ones
who do not.
