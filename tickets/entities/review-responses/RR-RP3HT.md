---
id: RR-RP3HT
type: review-response
title: Value type T=unknown will perpetuate FieldRenderer wart
finding: Generic T=unknown forces callers to cast. Current FieldRenderer is already value:unknown -- the registry could fix this or perpetuate it. After 9 widgets ship, narrowing later means touching all of them.
severity: minor
reason: Narrowing value type per widget (TextWidget extends WidgetProps<string>, etc.) is a real improvement but expands the ticket. T=unknown stays for this ticket; per-widget narrowing is a follow-up after the registry shape proves stable. Accepted as a known limitation, not a blocker — the registry contract supports tightening later without breaking existing widgets.
status: deferred
---
