---
id: RR-LJDWVVW
type: review-response
title: Add column offered where Configure cannot edit the schema
finding: 'With includes: or extends: or a namespaced type the control shows but the edit is refused only at click time.'
severity: significant
resolution: Config load now refuses add_column on a namespaced type. Includes and extends cannot be seen in the loaded metamodel. The control then shows the server or unsupported reason inline and the guide names both limits.
status: addressed
---
