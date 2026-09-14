---
id: RR-7DHGWF
type: review-response
title: Equality-guard test asserted on the wrong node and pinned nothing
finding: DocumentView.rerender.test.ts asserted node identity on .document-body, the one element v-html never replaces (it replaces the container's CONTENTS). Removing the equality guard entirely killed zero tests, so the guard had no coverage and my 'mutation-verified' claim covered only the cold-load half.
severity: critical
resolution: 'Probe changed to the container''s firstChild, plus a second test spying on renderMermaidDiagrams to assert the diagram watcher does not re-run on an unchanged render. Investigating further showed the deeper truth: assigning an identical string to a Vue ref does not trigger reactivity at all, so the guard is belt-and-braces rather than the mechanism preventing the repaint - verified with a standalone v-html probe component. The code comment claimed the opposite and was corrected in both files.'
status: addressed
---
