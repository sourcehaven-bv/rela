---
id: RR-YQ6LF4
type: review-response
title: 'Edit cap: the 10-target check must cover the union with kept hidden targets'
finding: 'AC 6 makes the new set `resolved ∪ (old ∩ candidates)`. resolved can hold 10 visible targets and the kept part can add hidden ones, so the union can exceed 10. The store then answers ErrTooLarge, which the plan does not map, so it becomes a 500. If the service checks the union instead, the 422 counts a target the editor can no longer see. Fix: run tooManyMentions on the final union in Service.Edit, and record that the extra bit is acceptable because the editor wrote that id while they could see it. Or keep hidden kept targets first and drop the excess visible ones. Add a TestEdit row for each choice.'
severity: minor
resolution: Service.Edit checks the 10 cap on the final union incl. kept hidden targets; 422 accepted since the editor wrote those ids; TestEdit_TooManyWithKeptHidden.
status: addressed
---
