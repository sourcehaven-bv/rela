---
id: RR-Y5WQRHQ
type: review-response
title: Trailing nil argument refused
finding: admin.update_entity(id, props, nil) is refused with a message about content and options.
severity: nit
reason: The extra argument is unusual and the error states the accepted form.
status: wont-fix
---
