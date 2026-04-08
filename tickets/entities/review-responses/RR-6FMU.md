---
id: RR-6FMU
type: review-response
title: Provider-divergence handling is hand-waved; concrete shapes not addressed
finding: 'Plan says ''treat optional fields as optional'' but only addresses missing usage. Real divergences in OpenAI-compat providers: (1) Ollama returns prompt_eval_count/eval_count instead of usage in some versions, omits both in others; (2) finish_reason may be absent or named done_reason; (3) LM Studio has returned choices[0].text instead of choices[0].message.content for some loaders; (4) some compat layers (Anthropic-style) return content as an array of content parts ({type:text, text:...}) — your string deserialize will fail at JSON unmarshal time or silently produce empty content. Fix: deserialize content as json.RawMessage, then handle both string and array-of-parts shapes. Add tests for: missing usage, missing finish_reason, content-as-array, alternative usage field names. If we cannot reasonably handle a shape, return a clear error naming the shape.'
severity: significant
resolution: 'Response decoding now uses json.RawMessage for choices[0].message.content. The decoder tries string first, then array of {type, text} parts (concatenating text fields), else returns ErrBadResponse with a clear message. usage, finish_reason, model are all optional — missing fields produce zero values, not errors. ACs #19 (missing optional fields), #20 (content-as-array), #21 (unrecognized content shape) cover this. Empty choices returns ErrBadResponse (AC #22).'
status: addressed
---
