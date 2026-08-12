---
id: RR-GHU9TE
type: review-response
title: preformatted was a two-source-of-truth flag derived at the call site
finding: Both call sites derived the value shape as `hint.kind === 'text'`. That encodes 'TextWidget does no formatting', but the real property is 'this widget has no display-mode formatter', which is true of text, integer, and text-list, and will be true of the forthcoming ColorWidget. It was already wrong for text-list, and each new widget would default to being mis-shaped.
severity: significant
resolution: Introduced DenseRoutingHint extending WidgetRoutingHint with preformatted, computed inside densePropertyRoutingHint from a PASSTHROUGH_KINDS set. Routing now owns both which widget and what shape it wants; call sites just read hint.preformatted. A new widget defaults to receiving the raw value, which is the correct default since the reason to add a widget is that it renders something text cannot.
status: addressed
---
