---
id: RR-0AVS9IR
type: review-response
title: 'set with the placeholder erases values'
finding: 'A set or create.properties value with {{current_user.id}} wrote an empty value for every write without a resolved user, erasing a stored value on update.'
resolution: 'Such a value is now left out when the event has no resolved user (lacksCurrentUser); engine tests and the guide cover it.'
severity: significant
status: addressed
---
