---
id: RR-24OLNIQ
type: review-response
title: RekeySubject could move runs it never locked on pg
finding: Rekey locked (flow, subject id) keys from a first read; a create of another flow on the old id could commit after it and be moved unchecked, or be checked without holding the new id's lock.
severity: significant
resolution: 'Locks are now per subject id: Create locks its subject id and RekeySubject locks the old and new ids in sorted order before any read. New conformance case ConcurrentRekey races a rekey against singleton creates of another flow on both ids.'
status: addressed
---
