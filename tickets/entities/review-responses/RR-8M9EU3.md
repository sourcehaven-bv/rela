---
id: RR-8M9EU3
type: review-response
title: min/max allowed on computed or rollup integers
finding: A derived value out of range warns on writes the user cannot fix directly.
severity: nit
reason: A range on a derived value is a useful check that its inputs make sense; the warning names the property and the user fixes the inputs. Refusing it would remove that check.
status: wont-fix
---
