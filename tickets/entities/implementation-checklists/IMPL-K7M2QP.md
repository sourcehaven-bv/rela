---
id: IMPL-K7M2QP
type: implementation-checklist
title: 'Implementation: Declarative scheduled mail with per-recipient ACL scoping'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration boundaries exercised through scheduler and appbuild tests
- [x] Happy path implemented
- [x] Empty results, invalid config, removed recipients, and bounded fan-out handled
- [x] Errors surfaced with task/template/recipient context

## Test Quality

- [x] Tests use narrow reader/workspace fixtures
- [x] Assertions cover only behavior relevant to the boundary
- [x] ACL test proves content can only originate from the visible reader

## Manual Verification

- [x] Feature manually tested end-to-end with a fixture project and memory sender
- [x] Each acceptance criterion verified against the final implementation

**Verification Evidence:** The appbuild fixture sends through the real renderer
and memory transport, asserting recipient, `RenderedFor`, and content. `just
test` passes under the race detector; mailtemplate and scheduler each cover 85%.

## Quality

- [x] Code follows existing scheduler, visibility, renderer, and sender seams
- [x] No second mail queue or persistent claim store introduced
- [x] Raw recipient access is restricted to envelope address lookup
- [x] No silent identity fallback or broadcast path
- [x] No debug code left behind
