---
id: DOCS-5FR0I2
type: docs-checklist
title: 'Docs: rela channel action and session'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (fakeActor says which states the real service cannot reach cheaply; actionText; sessionReadText)
- [x] Function/type docs if public API (ChannelActionCmd, ChannelSessionCmd and their Run methods; channelActor, channelSessionReader)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] Help text accurate (if CLI changes) (help for action, --release and session; the action arg points at the may: line of rela channel read)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-cli-reference.md § rela channel: synopsis, action, --release, session, and the may: line of read; regenerated docs/cli-reference.md)
