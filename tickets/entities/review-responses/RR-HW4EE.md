---
id: RR-HW4EE
type: review-response
title: mockReset wipes implementation; safer to mockClear + default
finding: 'searchSpy.mockReset() in beforeEach (CommandPaletteModal.test.ts:90) wipes implementation AND queued mockResolvedValueOnce. Today no test triggers a search without queuing a value, but a future test might silently break (resp.data on undefined). Fix: use mockClear() in beforeEach and add a default `searchSpy.mockResolvedValue(listResponse([]))` so missing queues don''t blow up.'
severity: minor
resolution: Switched beforeEach from searchSpy.mockReset() to searchSpy.mockClear() and added a default `searchSpy.mockResolvedValue(listResponse([]))` so missing per-test queues don't crash on resp.data.
status: addressed
---
