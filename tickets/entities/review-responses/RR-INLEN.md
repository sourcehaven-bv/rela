---
id: RR-INLEN
type: review-response
title: "widget on an inline enum ignored the value set"
severity: significant
status: addressed
finding: "An inline enum is {Type: string, Values: [...]}. The table saw only Type, so widget: text and widget: textarea were ACCEPTED on a value-constrained property. defaultWidgetFor routes values.length > 0 to select BEFORE consulting type, so the SPA's real dispatch key is (list, values, type) while the validator modelled only type. The effect is a free-text box over a constrained value set: the operator can type anything and PATCH it. Less severe than the list case (the write validator would 422 the bad value) but it is still config that validates and then does something the author did not ask for."
resolution: "Handled by the same widgetAcceptsProperty rewrite: a property with a fixed value set — inline values: or a custom type carrying values — accepts only the select family. Pinned by TestValidateConfig_WidgetOnInlineEnum."
---
