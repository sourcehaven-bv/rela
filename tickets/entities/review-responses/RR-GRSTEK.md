---
id: RR-GRSTEK
type: review-response
title: Newlines in trace text could inject Mermaid statements
finding: Error messages and identifier fields were unquoted; label() did not escape CR/LF/backtick.
severity: minor
resolution: 'summarize flattens CR/LF/NUL; label() replaces them and escapes backticks. Test: TestLabelOneLine.'
status: addressed
---
