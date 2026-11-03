---
id: RR-IH9XUE
type: review-response
title: State the outcome of the attachments check and pin the directory model
finding: 'The scope says the attachments index is "checked for the same pattern"; it has no persisted cache (loadAttachmentsIndex walks attachments/ on every open), so no follow-up is needed; say so. Also state the layout the per-directory grouping assumes: entity files are flat under entities/<plural>/, faces live in the file name (id@face.md), and the scan ignores subdirectories, so the grouping key is the directory name the scan listed, not a value derived from the entry. Note that buildPluralToTypeMap is last-wins on Go map order when two types share a plural, so a cached type for such a directory can differ between runs.'
severity: nit
resolution: 'Plan updated: the attachments index has no persisted cache, so no follow-up; the directory model no longer matters once the cache is gone.'
status: addressed
---
