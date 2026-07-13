---
id: RR-HOLGCD
type: review-response
title: Sub-minute precision dropped when editing (datetime-local is minute-granular)
finding: 'datetime-local input is minute-granular: a stored 12:30:45Z displays as 12:30 and re-emits 12:30:00Z when edited, dropping seconds. Non-destructive design protects untouched values, so only bites when a user edits a sub-minute-precision field. Inherent HTML-input limitation, not a bug. The optional :ss group in localInputToUtcISO''s regex is effectively dead for the widget. Fix: one-line doc note; no code change required.'
severity: nit
resolution: Documented. Added a note to the localInputToUtcISO docstring that the input is minute-granular so sub-minute precision on a pre-existing value is dropped when that field is edited (the optional :ss regex group is dead for the widget). No code change needed - inherent to <input type=datetime-local>, and the non-destructive design protects untouched values.
status: addressed
---
