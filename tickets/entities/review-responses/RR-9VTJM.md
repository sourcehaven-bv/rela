---
id: RR-9VTJM
type: review-response
title: 'Minor: asymmetric seq on create, overpromising watcher comment, hand-rolled itoa'
finding: 'go-architect minors: (a) seq is set via DEFAULT nextval on INSERT but explicitly bumped via nextval(''rela_seq'') on UPDATE/rename/attachment — asymmetric; harmless but worth a note that creates also consume the sequence via the column default. (b) pgstore.go Subscribe/emit doc could overpromise vs what emit guarantees (lossy, unordered) — align the comment with store.go:280 contract. (c) search.go has a hand-rolled itoa() helper that should just be strconv.Itoa.'
severity: minor
resolution: 'Implemented (commit 9daa5f84): search.go uses strconv.Itoa (hand-rolled itoa removed); watcher emit doc tightened to state lossy/unordered + lost-event window; rela_seq comment added to 0001_init.sql explaining INSERTs consume the sequence via the column DEFAULT while UPDATE/rename bump it explicitly.'
status: addressed
---

## Resolution plan

Cheap nits, fix in the review-fix pass:
- (c) Replace `itoa` in search.go with `strconv.Itoa` (the comment claiming
"keep imports minimal" is not worth a custom impl).
- (b) Tighten the Subscribe/emit doc comments to match the lossy/unordered
contract already documented on store.Watcher.
- (a) seq asymmetry: add a one-line comment that INSERTs consume rela_seq via the
column DEFAULT, so the explicit nextval() on UPDATE/rename is the matching bump
(not a missing-on-create bug). No behavior change.
