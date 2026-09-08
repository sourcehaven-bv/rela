---
id: DOCS-LD8RGZ
type: docs-checklist
title: 'Docs: app CSP requires external scripts and styles'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `appCSP`'s doc says what the missing `unsafe-inline` protects against and what it does NOT (it is not the data boundary); the scaffold template carries a comment telling anyone editing it why inlining is not an option
- [x] Function/type docs if public API — `ScaffoldApp` documents why it writes three files

## Project Documentation

- [x] ~~README updated (if applicable)~~ (README does not cover custom apps)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (no new pattern; tightens an existing header)
- [x] Help text accurate (if CLI changes) — `rela apps new` now lists all three files and notes that the CSP blocks inline code

## External Documentation

- [x] ~~Changelog entry added~~ (repo has no CHANGELOG.md)
- [x] API docs updated (if applicable) — the data-entry guide's Custom apps section gains "Scripts and styles must be separate files": what is blocked (inline `<style>`/`<script>`, `style=""`, `on*=`), what to write instead, and why. Edited in `docs-project/` and regenerated with `just docs`.
