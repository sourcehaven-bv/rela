---
id: TKT-9XDEY0
type: ticket
title: 'Extract attachmentStore off fsstore.FSStore: the on-disk attachment set as a closed type (92 → 84)'
kind: refactor
priority: medium
effort: s
tags:
    - tech-debt
status: backlog
---

Sub-ticket of [[TKT-N0IKN9]], continuing the `fsstore.FSStore` arc opened by
[[TKT-Y683LJ]] (`fileLayout` + `mdCodec`). First extraction after the self-echo
bug is settled — it must not bake the dead `RecordWrite` path in.

## Context: what the numbers actually mean here

Of FSStore's 36 exported methods, **31 are mandated** by `store.Store` (31
methods across its 10 embedded interfaces) and 2 are `store.Formatter`; only
`StartWatching`/`StopWatching` (already consumed via consumer-side interfaces)
and `RecordWrite` are surplus. The exported directive can move 36 → 31 at best.
**The ratchet target is the 56 unexported helpers**, and the question for each
extraction is "which fields form a closed set only reachable through a narrow
contract?" — not "how do I get under 40".

## The closed set

Fields `attachments`, `attachKey`, `streamingSupported` are touched only by
`attachment.go` (10 sites) and one loader in `fsstore.go`. Verified by grep.

## What moves (8 methods → `internal/store/fsstore/attachmentstore.go`)

`attachFile` (attachment.go:36), `writeAttachment` (:101), `deleteAttachment`
(:137), `removeAttachmentDir` (:185), `renameAttachmentDir` (:239),
`streamToFile` (:271), `loadAttachmentsIndex` (fsstore.go:299),
`loadPropertyAttachments` (:337).

```go
type attachmentStore struct {
    rooted    *storage.RootedFS
    key       string // root-relative, "" disables
    streaming bool
    metas     map[string]attachMeta // "entityID/property/fileName"
}
func (a *attachmentStore) load() error
func (a *attachmentStore) put(entityID, property, name string, r io.Reader) error
func (a *attachmentStore) open(entityID, property, name string) (io.ReadCloser, error)
func (a *attachmentStore) remove(entityID, property, name string) error
func (a *attachmentStore) list(entityID string) []store.AttachmentInfo
func (a *attachmentStore) removeEntity(entityID string) error
func (a *attachmentStore) renameEntity(oldID, newID string) error
```

FSStore keeps the 4 mandated `AttachmentManager` wrappers. The existence check
`s.entities[entityID]` (attachment.go:47) STAYS in the wrapper — that is index
knowledge, and keeping it out is what makes the type closed.

## Locking rule (applies to the whole fsstore arc)

**Extracted types take NO lock of their own.** `mu` (fsstore.go:195) guards
every index field with one flat lock; `deleteEntity` (entity.go:406-505) touches
index + attachments + observers + subscribers atomically under it. A per-type
mutex would fragment that into non-atomic steps and create a lock hierarchy. The
caller holds `s.mu`; document "must be called under mu" on the type. Readers
must never take `txMu` (fsstore.go:188-192;
`TestTx_ReadsViaOuterHandleDoNotDeadlock`).

## Sharing note

Not shareable across backends — memstore holds `[]byte`, pgstore BYTEA,
sqlitestore BLOB. Only `attachmentKey` (verbatim in `fsstore/attachment.go:31`
and `memstore/memstore.go:1085`) is worth hoisting to `storeutil`. Adjacent
drift worth a one-liner in this PR or a note: `pgstore/attachment.go:29-34`
open-codes the size cap with `io.LimitReader` and returns a bare error where the
others use `storeutil.LimitAttachmentReader` + `ErrAttachmentTooLarge`.

## Done when

`storetest.RunAll` conformance (`fsstore/conformance_test.go:52`) + the fuzz
corpus, especially `FuzzAttachmentKeyCollision`, run green — not just compiled.
`go test -race ./internal/store/...`. Ratchet `//plimsoll:max-methods`
(fsstore.go:159) to the new exact count.
