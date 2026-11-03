---
id: RR-GNLG1TQ
type: review-response
title: File schedule writes could zero Cause
finding: filechannels meta.setSession runs on every schedule write (non-moving posts, reminders, close, reopen); missing either side silently zeroes Cause. Meta keys are flat; SQL touches five places.
severity: minor
resolution: Plan names meta.channel, meta.setSession and the flat session_cause key, and chanCols, chanColCount, scanChannelWith, bumpSession and the insert on SQL; AC 1 adds reminder post, close, reopen and untombstone with resume.
status: addressed
---
