---
id: RR-7L0SW
type: review-response
title: 'Acceptance criterion #6 (SSE-during-edit) is too vague and would pass trivially today'
finding: |-
    Criterion says 'fires an SSE event after a local edit but before the field is committed'. But today nothing in the form responds to SSE — formData isn't going to be overwritten anyway, so the test passes trivially.

    Stronger version: mount form on E, type 'abc' into field X (do not advance debounce). Trigger SSE entity:updated for E. Mock API to return E with field X='serverValue'. Assert formData.X === 'abc' AND after debounce + PATCH success, formData.X reflects user value. Converse test: SSE for E, field Y not dirty, server value 'newY' — assert formData.Y === 'newY' after SSE-driven refresh. Latter only meaningful once SSE refresh is wired (see critical #1).
severity: minor
resolution: 'AC #9 rewritten with concrete assertions: mount, type ''abc'' into X (no debounce), trigger SSE for E with X=''serverValue'', assert formData.X === ''abc''; advance debounce; assert PATCH fires with user value. Converse path tests non-dirty Y refresh. Both halves only meaningful once the SSE refresh hook is wired (per RR-P7E24, also addressed).'
status: addressed
---
