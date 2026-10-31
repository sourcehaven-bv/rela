---
id: RR-QEEQ8JA
type: review-response
title: The error path must not yield unredacted or partial rows
finding: 'Design review: on a store error mid-page, the buffered rows must not be yielded raw, and a partial page could read as complete.'
severity: significant
resolution: redactingSeq drops the buffered page and yields only the error, as ScriptReader.ListEntityHeaders does. TestRedactingSeq_ErrorDropsTheBufferedPage.
status: addressed
---
