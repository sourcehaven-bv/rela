---
id: RR-4WG64Q
type: review-response
title: '@click.stop on the parent link does suppress the details toggle (finding refuted)'
finding: Review claimed that @click.stop on the title link inside <summary> cannot prevent the disclosure toggling, because <details> activation is the browser's default behaviour on summary rather than a bubbling listener, and stopPropagation does not cancel default behaviour -- so clicking the title would both navigate and toggle.
severity: minor
reason: 'Tested and refuted, not dismissed. Dispatching a real bubbling MouseEvent on the title link in the running app: openBefore=false, openAfter=false -- the details did NOT toggle. Dispatching the same event on the twisty: false -> true, so expansion still works. Chrome routes summary activation through the bubbling click, so stopPropagation does suppress it. My first probe (a standalone HTML file with element.click()) was unsound because programmatic .click() does not exercise summary activation at all; the in-app bubbling-event test is the valid one. The reviewer''s note that the child link correctly has no .stop is right, and the asymmetry is intentional: the child is not inside a summary, so it has nothing to suppress. Added a comment saying so, since the asymmetry reads as cargo-culting otherwise.'
status: wont-fix
---
