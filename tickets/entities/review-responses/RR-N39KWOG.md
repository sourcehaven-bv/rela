---
id: RR-N39KWOG
type: review-response
title: Hook determinism and argument order
finding: A hook could change the key or be nondeterministic; argument order differed between documents.
severity: minor
resolution: The hook signature is function(draft, record); a changed key is a record error; the Lua state is bound to the command context.
status: addressed
---
