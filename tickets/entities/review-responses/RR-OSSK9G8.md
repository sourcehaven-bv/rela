---
id: RR-OSSK9G8
type: review-response
title: Terminal sanitization too narrow
finding: Control-character checks let bidi overrides and other format characters through. Quote non-printable characters in text output for local and remote values.
severity: minor
resolution: reconclient.Sanitize replaces non-printable and Cf runes and truncates remote text; addresses must be Printable; the text output quotes any value that is not safe to print.
status: addressed
---
