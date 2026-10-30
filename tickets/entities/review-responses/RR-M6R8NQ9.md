---
id: RR-M6R8NQ9
type: review-response
title: Section edit forms save outside the session
finding: SectionEditForm autosave does not go through session.write, so a stale view can land over a section save and revert the form, or mount a form with old versions that then reports a false conflict.
severity: significant
resolution: SectionEditForm takes an optional runWrite prop; EntityDetail passes the session write at all three render sites.
status: addressed
---
