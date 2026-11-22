---
id: AM-mail-branding-and-text-addresses
type: automated-measure
title: App branding in rendered mail and email addresses in the text part
description: mailbrand, mailrender, lua and scheduled-mail tests pin that mail uses the app accent and app.logo with defaults as fallback, and that the text part keeps email addresses.
kind: test
location: internal/mailbrand/mailbrand_test.go, internal/mailrender/mailrender_test.go, internal/lua/mailrender_test.go, internal/appbuild/scheduled_mail_test.go
status: active
---

Fails when a caller renders mail without the app's branding, when a logo mail
clients cannot show is linked, or when the text part drops an email address.
