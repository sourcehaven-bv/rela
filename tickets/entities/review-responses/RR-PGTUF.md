---
id: RR-PGTUF
type: review-response
title: 'Minor: watcher lost-event doc, --database-url advertised on all builds, ctx-less search, DSN redaction, validateProperty parity'
finding: 'cranky-code-reviewer #5-#9: (#5) watcher emit is post-commit non-atomic — a crash between commit and emit loses the event for in-process subscribers; the doc only frames the duplicate direction (''never observe uncommitted''), not the lost direction. Acceptable for single-writer (consumers re-snapshot on reconnect) but doc should say so. (#6) --database-url flag + help appear on the default FS build where it''s silently ignored. (#7) SearchBackend.Search uses context.Background() (forced by search.Backend interface having no ctx) — hung search survives request cancellation. (#8) open.go comment asserts pgx redacts password; verify the Migrate-path connection-failure error doesn''t embed the DSN before trusting it in slog. (#9) validateProperty only called by AttachFile, not relation/entity property writes — confirm parity with memstore is intentional.'
severity: minor
resolution: '(#5) Done — pgstore.go watcher godoc now states the lost-event window explicitly (crash-between-commit-and-emit / full buffer) and that consumers re-snapshot on reconnect; the ''never observe uncommitted'' framing is no longer one-directional. (#7) Done — search.go has a comment at the context.Background() call pointing at the search.Backend interface having no ctx param (interface change tracked separately). (#8) Verified — forced a real auth failure (nonexistent role with a password) against the live server: error shows `user=nouser database=rela_test` with the password NOT present (leak count 0); pgx redacts on the connect path, not just parse. No defensive redaction needed. (#9) Verified parity — memstore also calls validateProperty ONLY in AttachFile (memstore.go:662), not on entity/relation property writes, so pgstore matches exactly; intentional. (#6) Deferred — the --database-url flag appears on the default build but its help says ''postgres build only'' and it''s a no-op there (WithDatabaseURL ignores empty / FS build ignores the field). Low value; a startup warning could be a follow-up. Not a correctness issue.'
status: addressed
---

## Resolution plan

Fix the cheap doc/UX ones now; note the interface-limited ones:
- (#5) Tighten watcher godoc to state the lost-event window explicitly + that
recovery relies on consumer re-snapshot (dataentry/MCP do on reconnect).
- (#8) Verify pgx connection-failure error redaction with a quick test (invalid
password DSN -> assert error string has no password); already saw `user=u
database=nope` redacts in the Stage-B e2e, but confirm for the connect (not just
parse) path. Defensive redaction if needed.
- (#6) Low priority: a startup log when a DSN is supplied to a non-postgres
build would help, but the flag help already says 'postgres build only'. Consider
deferring.
- (#7) ctx-less Search is a search.Backend interface limitation (no ctx param);
add a code comment pointing at it; interface change is a separate ticket.
- (#9) Confirm via conformance that entity/relation property-name validation
parity with memstore is intentional (memstore also only validates attachment
property names via storeutil.ValidateProperty) — likely already matching.
