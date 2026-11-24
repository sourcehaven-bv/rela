---
id: RR-114Z5BE
type: review-response
title: 'Code: a cleared dependent is not restored when the scope returns'
finding: 'With on_change clear, changing the scope and back left the dependent empty.'
severity: minor
resolution: 'DynamicForm remembers the relations it cleared and restores the saved value when the scope returns to its saved value, unless the user edited the dependent.'
status: addressed
---
