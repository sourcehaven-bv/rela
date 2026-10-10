---
id: RR-LLMA0W
type: review-response
title: Moving between handles drops the keyboard preview
finding: Blur from start to end cleared the preview, so start and end could not be saved together.
severity: minor
resolution: Blur keeps the preview when focus moves to another handle of the same bar (relatedTarget in the same drag window). Test saves start and end in one write. Documented.
status: addressed
---
