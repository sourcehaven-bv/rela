---
id: RR-3H9RSI
type: review-response
title: '[security] fs watcher can add a face between re-read and delete'
finding: The fsstore watcher indexes external file edits without the Tx lock, so a face can appear after the in-Tx re-read.
severity: minor
resolution: The store's DeletedEntities is authorized after the delete; on fs the removed faces are recorded, on pg/sqlite the Tx rolls back. Pinned by the before-store-delete test.
status: addressed
---
