---
id: AM-feed-field-redaction
type: automated-measure
title: 'The ICS feed omits visible:-hidden properties, and redaction runs after the filter'
description: 'TestDeclarativeFeed_RedactsHiddenProperties asserts a hidden property never reaches a rendered event. TestDeclarativeFeed_RedactionDoesNotChangeMembership pins the ORDER by hiding the property the where: clause filters on and asserting the event still appears - moving redaction before the filter drops it, so feed membership would vary per principal. TestDeclarativeFeed_RedactionCopies asserts the shared store entity is not mutated. All three verified to fail against the fix being removed.'
kind: test
location: internal/dataentry/feed_provider_test.go
status: active
---
