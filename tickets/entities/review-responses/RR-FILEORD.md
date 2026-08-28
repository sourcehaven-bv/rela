---
id: RR-FILEORD
type: review-response
title: "widget: file check ran before the type check, hiding the better error"
severity: minor
status: addressed
finding: "The WidgetFile display-mode branch ran first and continued, so widget: file on a string property in a cards section reported only the display-mode problem. Fixing the display mode would surface a second, different error on the next load — two round-trips for one config line. The existing test used property: title (a string) and so exercised this double-fault while asserting only the first message."
resolution: "Reordered so property-type compatibility is checked first, and gave the file-widget test a genuine file-typed property so it isolates the display-mode rule. Added a third case asserting the type error wins when a field violates both."
---
