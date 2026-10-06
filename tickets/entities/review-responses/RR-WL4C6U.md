---
id: RR-WL4C6U
type: review-response
title: Helper takes both label and plural
finding: createFromMenu needs the menu label and the API plural.
severity: minor
resolution: Not changed.
reason: The menu label and the API plural are independent names in config; deriving one from the other would encode a naming convention the fixture does not guarantee. Two explicit arguments keep the helper honest.
status: wont-fix
---
