---
id: RR-YA3WLA
type: review-response
title: Preconditions miss changes made since the chart loaded
finding: The commit took values and versions from a fresh read, so the precondition only guarded the GET-to-PATCH gap; a date changed by someone else was silently shifted.
severity: significant
resolution: Before writing, each fresh value's day must equal the preview origin; otherwise the write is refused with a conflict message and the chart reloads. Test added; docs reworded.
status: addressed
---
