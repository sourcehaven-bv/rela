---
id: RR-BWXS
type: review-response
title: getTokenAt(precise=true) is fine but the bare 'precise' flag is undocumented in the wrapper interface
finding: 'useBacktickAutocomplete.ts CodeMirrorLike interface line 96 declares `getTokenAt(pos, precise?: boolean)` and the call sites pass `true` (lines 349, 356). Per CodeMirror v5 docs, `precise=true` forces synchronous re-tokenization — so the worry about a stale buffer state at the moment `inputRead` fires is mitigated. However: the doc-comment on `CodeMirrorLike` doesn''t say why `precise` is required, only that it''s the second argument. A future maintainer who refactors the wrapper or skips the second argument would silently introduce stale-tokenizer races. Recommend: rename `precise` to a more descriptive name (or wrap in a `getTokenAtSync(pos)` helper) and document the requirement at the call site. Lower the risk that someone deletes the `true` thinking it''s a tracing flag.'
severity: minor
resolution: Added a comment in CodeMirrorLike's getTokenAt signature noting that the precise=true flag is required to force synchronous re-tokenization.
status: addressed
---
