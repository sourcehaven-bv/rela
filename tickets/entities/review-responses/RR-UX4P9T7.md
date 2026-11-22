---
id: RR-UX4P9T7
type: review-response
title: "A narrow table between 640px and its floor scrolls sideways"
finding: "With the breakpoint at 640px, a table whose floors add up to more than 640px scrolls sideways until it reaches them instead of compressing."
severity: minor
reason: "Container queries cannot read the computed floor. Before the change every table scrolled up to 240px plus 150px per column above 768px, so the range is smaller than it was."
status: wont-fix
---
