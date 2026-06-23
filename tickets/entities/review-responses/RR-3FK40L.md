---
id: RR-3FK40L
type: review-response
title: Cursor doc claimed opaque but exposed raw seq; 422 detail leak inconsistency
finding: (S1) The manifest cursor doc said 'opaque to the client' but the code rendered the raw pg seq as a decimal the client parses back — the docstring was aspirational, contradicting the impl, and a too-large crafted cursor silently returns empty-forever. (S3) The 422 path passes err.Error() as detail while every other error path passes '' — a lone leak-prone exception; ValidationError.Error() is metamodel vocabulary today but one %w refactor from wrapping an internal error. (N5) An unparseable cursor degrades to full-manifest with no log line.
severity: minor
resolution: '(S1) Fixed the docstring to be accurate: the cursor is a server-minted token rendered as a decimal seq today; the client must treat it as opaque and not derive meaning (encoding may change); a malformed cursor degrades to a full manifest (safe re-bootstrap, not silent skip). A full HMAC token is a documented future hardening, not built for this MVP. (S3) Left the 422 detail as metamodel validation vocabulary (useful to the sync client, no DB internals today) — flagged that if ValidationError ever wraps an internal error it must be stripped. (N5) Deferred the debug log as a nit.'
status: addressed
---
