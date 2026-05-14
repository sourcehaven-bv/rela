---
id: RR-NKV5
type: review-response
title: Inserting `<id>` next to existing backticks corrupts the code span
finding: 'Plan inserts the bare text `\`<id>\``. If the cursor sits immediately adjacent to existing backticks (e.g. inside an already-open code span, or right after a closing backtick), the result is a syntactically broken code span — e.g. `\`abc\`\`TKT-1\`` parses as one big code span. The plan does not address context-aware insertion. Mitigation options: (a) detect adjacent backticks at the cursor position and prepend/append a space, (b) refuse to insert when inside an existing inline code span (CodeMirror token stream tells us), (c) just document this as a known limitation and let the user fix it. Pick one; add a test case. Without an explicit decision, implementation will guess.'
severity: significant
resolution: 'Plan §Approach §3: helper reads adjacent chars; if a backtick sits immediately on either side, pad that side with a single space. AC 5 + helper tests cover cursor right-before/right-after/between backticks.'
status: addressed
---
