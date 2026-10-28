---
id: RR-CKNHVA
type: review-response
title: User entity rename leaves rows and markers on the old id
finding: 'Piles move owners on a person rename (pilePersonType), but rows keep the old target, so an open mention is lost after a rename. A later entity reusing the id inherits the pointers. That leaks no content, because the entry is re-gated, but it does show author and channel for channels the new holder may read. Fix: list rename next to delete in the retention table, and add it to the slice 2 forget-reader follow-up, so markers and rows move together on a person rename or delete.'
severity: minor
resolution: Rename and delete rows added to the retention table; one line for TKT-6V8K8I written in the follow-up file.
status: addressed
---
