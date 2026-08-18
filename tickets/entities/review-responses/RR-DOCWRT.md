---
id: RR-DOCWRT
type: review-response
title: Branch claims a document render cannot mutate; it can (write bindings are unguarded)
finding: 'Five places on this branch claim a document render cannot mutate. That is false.


  runDocumentScript (list_document.go:85) builds a WriterRuntime via NewWriterRuntime -> lua.NewWriter
  -> newRuntime(allowWrites=true), and registerBindings has NO isDocument guard. So rela.create_entity,
  update_entity, delete_entity, create_relation and write_file are present and callable on EVERY document
  script. Verified end-to-end through the real handler: a plain GET /api/v1/_documents/{name} ran rela.create_entity
  and it succeeded.


  The condition is PRE-EXISTING (reproduces on develop; filed this session as TKT-PX5YL7) and this branch
  does not introduce it. What this branch does is build a load-bearing safety argument on top of it and
  state that argument as fact:


  - document.go:186-188 "the render is structurally unable to mutate"

  - runtime.go:1948-1952 "makes ''a render cannot write'' structural"

  - dataentryconfig/config.go AllowACLBypass godoc: "a render is a GET" presented as prevented

  - internal/dataentry/CLAUDE.md: same, stated as a rule

  - elevated_document_test.go:277-279 "The structural guarantee: a render cannot mutate"


  What is actually true is narrower, and still worth having: the elevated `admin` handle carries no write
  methods, so an elevated script cannot write PAST THE ACL. Ordinary rela.* writes remain available, bounded
  by the caller''s ACL.


  The validation rule rejecting write/read+write on a document stays desirable (do not widen the hole),
  but its stated justification is fiction.


  This violates the root CLAUDE.md rule on gates directly: "write down which of the two you mean, because
  the next person will build on a secrecy property that was never real." I wrote down the stronger property.


  Undocumented consequence: permission: on an elevated document grants "may read whatever this script
  reads" (documented) AND silently "this script may write as you" (documented nowhere).'
resolution: |
  Corrected all five claims to what is true. The elevated `admin` handle carries no write methods, so an elevated script cannot write PAST THE ACL; that is the real property and it is worth having. "A render cannot mutate" is not true and no longer claimed anywhere.
  
  Fixed in: document.go (elevatedDeps godoc), runtime.go (newElevatedHandle), dataentryconfig/config.go (AllowACLBypass godoc), internal/dataentry/CLAUDE.md (new bullet stating the gap explicitly and instructing "say 'cannot write past the ACL'"), elevated_document_test.go (the "structural guarantee" case comment), and both user-facing guides (GUIDE-data-entry, GUIDE-lua-scripting) which now say the rule prevents a render mutating BEYOND the caller's permissions rather than implying rendering is read-only.
  
  The validation rule rejecting write/read+write is retained, restated as "refuses to widen an existing gap" rather than "closes one".
  
  Incidental confirmation while writing the new integration test: lua.NewWriter PANICS without an EntityManager, so a document render cannot even be constructed as a read-only runtime today. That is the clearest possible evidence for the underlying issue, and it is noted in the denyMutator godoc.
  
  The underlying condition remains open as TKT-PX5YL7 (filed earlier this session, before this review), which the corrected comments now reference by id.
severity: critical
status: addressed
---
