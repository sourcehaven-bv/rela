---
id: RR-DCIEBW
type: review-response
title: Ledger mutex held during channel-store I/O
finding: restore holds the ledger mutex while Untombstone waits on the channel file lock with no deadline, blocking every manager create in the process.
severity: minor
status: open
---
