---
id: RR-BRWHEY
type: review-response
title: Holder nil contract text is misleading
finding: A nil holder in the deps interface still registers the binding.
severity: nit
resolution: 'The Nil: line now says the method accepts nil but the binding is still registered, so the constructors never leave it nil.'
status: addressed
---
