---
id: RR-RO0WHTV
type: review-response
title: "The Settings-page logo is not used in mail"
finding: "An operator who uploaded a logo on the Settings page but set no app.logo gets mail without a logo."
severity: minor
reason: "The uploaded logo is served only to a signed-in user, so a mail client cannot fetch it by URL; embedding it would need mail.send to attach a part, which scripts do not ask for. app.logo is the public logo and is documented as the mail logo."
status: wont-fix
---
