---
id: RR-FS7A1Y
type: review-response
title: Web-app docs say the dialog is red only for a cancelled or failed state
finding: 'GUIDE-realtime.md, In the web app (the bullet that ends ''No other decision asks, unless the type''s decision_button says otherwise''), still says the confirm button is red when the state ends as cancelled or failed. Since this change it is also red when the button has style: danger (useSessionDecisions.ts:307). The bullet also says only decision_button can make a decision ask, but a preset''s own confirm can too. Fix: say ''or when the button''s style is danger'', and name the preset''s button: as well as decision_button. Run just docs.'
severity: minor
resolution: 'Web-app bullet now says the confirm button is red when the state ends as cancelled or failed, or when the button''s style is danger, and names a preset''s button: as well as decision_button. just docs run.'
status: addressed
---
