---
id: RR-DO1FZ2
type: review-response
title: Rename hook can detach messages posted to the new id after commit
finding: Between rename commit and Tombstone(newID), a post to newID creates a channel that the tombstone then detaches. Undocumented window.
severity: minor
resolution: Window documented on EntityRenamed; closing it needs one transaction across store and channels, which the store does not offer.
status: addressed
---
